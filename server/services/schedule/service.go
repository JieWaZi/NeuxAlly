package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	"github.com/JieWaZi/NeuxAlly/server/internal/schedulecalc"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	workflowfiles "github.com/JieWaZi/NeuxAlly/server/internal/workflow/files"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	channelservice "github.com/JieWaZi/NeuxAlly/server/services/channel"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/NeuxAlly/server/services/execution"
	"github.com/JieWaZi/NeuxAlly/server/services/workflow"
)

// Executions 是自动入口共用的标准执行和保存前校验端口。
type Executions interface {
	// Start 以可靠接纳模式启动一个标准 Chat。
	Start(context.Context, execution.WorkIntent) (execution.AcceptedExecution, error)
	// ValidateConfiguration 校验运行时和权限组合。
	ValidateConfiguration(context.Context, execution.ExecutionConfiguration) error
}

// Chats 只读取首次执行权威状态及原子删除直接关联的 Chat。
type Chats interface {
	// GetChatSummary 读取 Worker 实际工具授权和输入等待状态。
	GetChatSummary(context.Context, string) (chat.ChatSummary, error)
	// ReadInitialExecution 读取首次执行及当前审批和输入事实，不投影消息正文。
	ReadInitialExecution(context.Context, string) (chat.InitialExecutionState, error)
	// DeleteChatsAtomically 在同一事务中提交 Chat 及任务聚合的删除。
	DeleteChatsAtomically(context.Context, []string, func(context.Context) error) error
}

// Teams 只负责把冻结机会交给既有团队首发用例。
type Teams interface {
	// SubmitMessage 以稳定首次发送身份建立团队会话。
	SubmitMessage(context.Context, channelservice.SubmitMessageInput) (channelservice.SubmitMessageResult, error)
}

// Workflows 复用工作流接纳和权威状态，不在调度服务内运行流程。
type Workflows interface {
	// UploadFile 保存工作流目标的任务级上传原件。
	UploadFile(context.Context, string, string, string, io.Reader) (workflowfiles.Upload, error)
	// OutputFiles 读取固定版本声明的真实最终文件。
	OutputFiles(context.Context, models.WorkflowRun) ([]workflow.OutputFile, error)
	// OpenOutputFile 打开本次运行已声明的文件产物。
	OpenOutputFile(context.Context, string, string, string) (*os.File, string, error)
	// ValidateTargetConfiguration 校验目标与执行资源，允许本次输入尚未提供。
	ValidateTargetConfiguration(context.Context, string, string, models.WorkflowExecutionDefaults) (models.WorkflowDefinition, models.WorkflowRevision, error)
	// ReadTarget 读取说明所需的工作流与不可变输入契约。
	ReadTarget(context.Context, string, string) (models.WorkflowDefinition, models.WorkflowRevision, error)
	// StartRun 以机会的稳定身份接纳工作流。
	StartRun(context.Context, workflow.StartRunCommand) (models.WorkflowRun, error)
}

// Config 提供自动任务需要的稳定依赖，时钟可用于确定性验收。
type Config struct {
	// WorkflowRuns 读取工作流真实运行、操作许可和节点状态。
	WorkflowRuns controllers.WorkflowRunController
	// APIHandler 由 API 层提供本机 HTTP 适配器，服务只拥有监听生命周期。
	APIHandler LocalHandlerFactory
	// Workflows 接纳冻结版本的工作流并读取真实状态。
	Workflows Workflows
	// Teams 复用团队首发、成员快照和任务编排。
	Teams Teams
	// Channels 读取团队负责人和成员默认执行配置。
	Channels controllers.ChannelController
	// Threads 查询团队首发幂等身份、成员会话和活跃工作。
	Threads controllers.ChannelThreadController
	// TeamTasks 读取团队运行仍待处理或失败的成员任务。
	TeamTasks controllers.ChannelTaskController
	// Tasks 持久保存配置、运行槽和唯一机会。
	Tasks controllers.AutomaticTaskController
	// Transactions 提供机会领取及聚合删除的事务边界。
	Transactions controllers.TransactionController
	// Wakers 验证任务归属仍存在。
	Wakers controllers.WakerController
	// Projects 验证伙伴项目及已授权真实目录。
	Projects controllers.ProjectController
	// Executions 接纳标准执行并验证执行配置。
	Executions Executions
	// Chats 读取权威运行状态及执行聚合删除。
	Chats Chats
	// DataDirectory 保存任务 API 监听偏好。
	DataDirectory string
	// ManagedFilesDirectory 保存工作流上传文件，独立于任务 API 监听配置。
	ManagedFilesDirectory string
	// Now 返回当前时间，未提供时使用系统时钟。
	Now func() time.Time
}

// Service 在 Core 生命周期内协调任务接纳、计划机会与独立本机监听器。
type Service struct {
	// config 保存构造时固定的依赖。
	config Config
	// gate 串行保护配置、接纳及删除边界，不覆盖 Runtime 的实际执行。
	gate sync.Mutex
	// life 保护后台上下文、监听器及关闭等待的登记。
	life sync.Mutex
	// portGate 串行保护监听配置替换及关闭，避免并发设置使用同一临时文件。
	portGate sync.Mutex
	// ctx 是显式 Start 后生效的 Core 子上下文。
	ctx context.Context
	// cancel 取消后台扫描和所有正在接纳的请求。
	cancel context.CancelFunc
	// workers 等待扫描和本机监听器完全退出。
	workers sync.WaitGroup
	// suspended 表示桌面已报告休眠，期间不接纳定时机会。
	suspended bool
	// lastTick 是前一次连续运行的扫描时间，用于检测时钟跳变。
	lastTick time.Time
	// listener 保存已绑定的套接字，确保 Serve 尚未开始时也能同步释放。
	listener net.Listener
	// server 是独立于 Renderer 管理接口的本机 HTTP 服务。
	server *http.Server
	// api 保存地址、端口及监听结果的安全展示状态。
	api APIStatus
	// rates 保存每个任务最近一分钟已接纳 API 请求的时间。
	rates map[string][]time.Time
	// sealedWakers 封闭伙伴聚合删除期间的新任务及新接纳。
	sealedWakers map[string]bool
}

// New 构造尚未启动的自动任务服务。
func New(config Config) *Service {
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Service{api: APIStatus{IP: constants.AutomaticTaskDefaultAPIIP, Port: constants.AutomaticTaskDefaultAPIPort}, config: config, rates: map[string][]time.Time{}, sealedWakers: map[string]bool{}}
}

// APIStatus 是可持久修改的监听地址、端口及当前监听状态。
type APIStatus struct {
	// IP 是任务 API 监听的 IPv4 或 IPv6 地址。
	IP string `json:"ip"`
	// Port 是任务 API 监听端口。
	Port int `json:"port"`
	// Available 表示当前已经成功监听配置地址。
	Available bool `json:"available"`
	// ErrorCode 是端口冲突等失败的稳定原因，不含操作系统细节。
	ErrorCode string `json:"errorCode,omitempty"`
}

// LocalHandlerFactory 把 API 层的真实 HTTP 入口注入后台监听器。
type LocalHandlerFactory func(*Service, Config, string) http.Handler

// mutationScope 只在一次同步调用范围内证明当前服务的协调锁归属。
type mutationScope struct {
	// owner 指向本次持锁的调度组件，禁止跨服务复用保护。
	owner *Service
	// active 在释放前为真，释放后的 Context 不能再次绕过加锁。
	active atomic.Bool
}

type mutationScopeKey struct{}

// AcquireMutation 在完整 API 用例期间封闭配置、接纳和删除的竞争。
// 返回的 Context 只供本次同步调用及其事务使用，release 可以重复调用。
func (s *Service) AcquireMutation(ctx context.Context) (context.Context, func()) {
	if scope, ok := ctx.Value(mutationScopeKey{}).(*mutationScope); ok && scope.owner == s && scope.active.Load() {
		return ctx, func() {}
	}
	s.gate.Lock()
	scope := &mutationScope{owner: s}
	scope.active.Store(true)
	return context.WithValue(ctx, mutationScopeKey{}, scope), func() {
		if scope.active.CompareAndSwap(true, false) {
			s.gate.Unlock()
		}
	}
}

// ForgetTask 释放任务聚合删除后已无归属的进程限流记录。
func (s *Service) ForgetTask(ctx context.Context, taskID string) {
	_, release := s.AcquireMutation(ctx)
	defer release()
	delete(s.rates, taskID)
}

// Start 先对账未完成接纳并聚合错过计划，再显式启动扫描和本机监听器。
func (s *Service) Start(ctx context.Context) error {
	if s.config.APIHandler == nil {
		return status_error.AutomaticTaskServiceInvalid
	}
	s.life.Lock()
	if s.ctx != nil {
		s.life.Unlock()
		return status_error.AutomaticTaskServiceInvalid
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.life.Unlock()
	s.gate.Lock()
	err := s.recover(s.ctx)
	if err == nil {
		err = s.advanceMissed(s.ctx, s.config.Now())
	}
	s.lastTick = s.config.Now()
	s.gate.Unlock()
	if err != nil {
		s.cancel()
		return err
	}
	if err := s.loadAPISettings(); err != nil {
		s.api.ErrorCode = util.ErrorCode(err, status_error.AutomaticTaskServiceInvalid.Key())
	} else {
		s.listen()
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				if err := s.Tick(s.ctx); err != nil && s.ctx.Err() == nil {
					logx.Errorf("scan automatic task schedules failed: error_code=%s", util.ErrorCode(err, status_error.AutomaticTaskServiceInvalid.Key()))
				}
			}
		}
	}()
	return nil
}

// Close 先取消接纳和扫描，再关闭监听器并等待后台工作退出。
func (s *Service) Close() {
	s.portGate.Lock()
	defer s.portGate.Unlock()
	s.life.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	server, listener := s.server, s.listener
	s.server = nil
	s.listener = nil
	s.api.Available = false
	s.life.Unlock()
	if server != nil {
		_ = server.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	s.workers.Wait()
}

func (s *Service) recover(ctx context.Context) error {
	tasks, _, err := s.config.Tasks.List(ctx, controllers.AutomaticTaskFilter{})
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Deleting {
			if err := s.config.Tasks.SetDeleting(ctx, task.ID, false); err != nil {
				return err
			}
		}
		if task.ActiveRunID == "" {
			continue
		}
		run, err := s.config.Tasks.GetRun(ctx, task.ActiveRunID)
		if err != nil {
			return err
		}
		if err := s.ReconcileRun(ctx, &run); err != nil {
			return err
		}
		if run.Admission == constants.AutomaticRunAdmissionPending {
			if run.Source == constants.AutomaticRunSourceSchedule {
				if err := s.interrupt(ctx, &run); err != nil {
					return err
				}
			} else if err := s.config.Tasks.ReleaseRun(ctx, task.ID, run.ID); err != nil {
				return err
			}
		} else if err := s.refreshSlot(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

// Tick 处理连续运行期间的到期机会；时钟或调度大幅跳变时只推进错过游标。
func (s *Service) Tick(ctx context.Context) error {
	ctx, release := s.AcquireMutation(ctx)
	defer release()
	if s.suspended || ctx.Err() != nil {
		return nil
	}
	now := s.config.Now()
	gap := now.Sub(s.lastTick)
	wallGap := time.Duration(now.UnixMilli()-s.lastTick.UnixMilli()) * time.Millisecond
	if !s.lastTick.IsZero() && (gap > time.Minute || wallGap-gap > 2*time.Second || wallGap < 0) {
		s.lastTick = now
		return s.advanceMissed(ctx, now)
	}
	s.lastTick = now
	tasks, err := s.config.Tasks.Due(ctx, now.UnixMilli())
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if s.sealedWakers[task.WakerID] {
			continue
		}
		if now.UnixMilli()-*task.NextRunAt > constants.AutomaticTaskScheduleGraceSeconds*1000 {
			if err := s.missTask(ctx, task, now); err != nil {
				return err
			}
			continue
		}
		rule, err := schedulecalc.Compile(task.Schedule)
		if err != nil {
			return err
		}
		planned := *task.NextRunAt
		next := timePointer(rule.Next(now))
		key := fmt.Sprintf("schedule:%d:%d", task.TriggerVersion, planned)
		_, err = s.Admit(ctx, AdmissionCommand{Task: task, Source: constants.AutomaticRunSourceSchedule, OccurrenceKey: key, RequestID: fmt.Sprint(planned), ScheduledAt: &planned, NextRunAt: next})
		if errors.Is(err, status_error.AutomaticTaskBusy) {
			if err := s.skipBusy(ctx, task.ID, planned, next, key); err != nil {
				return err
			}
		} else if err != nil {
			// 已登记的接纳失败推进了游标，后续计划继续运行，不重试这一机会。
			logx.Errorf("admit scheduled automatic task failed: task_id=%s error_code=%s", task.ID, util.ErrorCode(err, status_error.AutomaticTaskServiceInvalid.Key()))
		}
	}
	return nil
}

func (s *Service) skipBusy(ctx context.Context, taskID string, planned int64, next *int64, key string) error {
	task, err := s.config.Tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	task.NextRunAt = next
	now := s.config.Now().UnixMilli()
	return s.config.Transactions.Transaction(ctx, func(tx context.Context) error {
		advanced, err := s.config.Tasks.AdvanceSchedule(tx, task, task.Revision)
		if err != nil {
			return err
		}
		if !advanced {
			return status_error.AutomaticTaskRevisionConflict
		}
		return s.config.Tasks.CreateRun(tx, models.AutomaticRun{ID: util.New(), TaskID: task.ID, WakerID: task.WakerID, OccurrenceKey: key, Source: constants.AutomaticRunSourceSchedule,
			Configuration: task.AutomaticTaskConfiguration, ConfigurationRevision: task.ConfigurationVersion, Admission: constants.AutomaticRunAdmissionSkipped,
			ErrorCode: status_error.AutomaticTaskBusy.Key(), ScheduledAt: &planned, CreatedAt: now, UpdatedAt: now})
	})
}

func (s *Service) advanceMissed(ctx context.Context, now time.Time) error {
	tasks, err := s.config.Tasks.Due(ctx, now.UnixMilli())
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := s.missTask(ctx, task, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) missTask(ctx context.Context, task models.AutomaticTask, now time.Time) error {
	rule, err := schedulecalc.Compile(task.Schedule)
	if err != nil {
		return err
	}
	count, last, next := rule.Missed(time.UnixMilli(*task.NextRunAt), now)
	task.MissedCount += count
	task.LastMissedAt = timePointer(last)
	task.NextRunAt = timePointer(next)
	_, err = s.config.Tasks.AdvanceSchedule(ctx, task, task.Revision)
	return err
}

// SetSuspended 接入桌面休眠和恢复，恢复时跳过所有过去的计划机会。
func (s *Service) SetSuspended(ctx context.Context, suspended bool) error {
	s.gate.Lock()
	defer s.gate.Unlock()
	s.suspended = suspended
	s.lastTick = s.config.Now()
	if !suspended {
		return s.advanceMissed(ctx, s.lastTick)
	}
	return nil
}

func (s *Service) begin(ctx context.Context) (context.Context, func(), error) {
	s.life.Lock()
	defer s.life.Unlock()
	if s.ctx == nil || s.ctx.Err() != nil {
		return nil, nil, status_error.OperationCanceled
	}
	linked, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	s.workers.Add(1)
	return linked, func() { stop(); cancel(); s.workers.Done() }, nil
}

func (s *Service) loadAPISettings() error {
	s.api = APIStatus{IP: constants.AutomaticTaskDefaultAPIIP, Port: constants.AutomaticTaskDefaultAPIPort}
	data, err := os.ReadFile(filepath.Join(s.config.DataDirectory, "automatic-tasks-api.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var saved APIStatus
	if err != nil || json.Unmarshal(data, &saved) != nil || net.ParseIP(saved.IP) == nil || saved.Port < 1 || saved.Port > 65535 {
		return status_error.AutomaticTaskAPIUnavailable
	}
	s.api.IP, s.api.Port = net.ParseIP(saved.IP).String(), saved.Port
	return nil
}

func listenAddress(ip string, port int) (net.Listener, error) {
	network := "tcp6"
	if net.ParseIP(ip).To4() != nil {
		network = "tcp4"
	}
	return net.Listen(network, net.JoinHostPort(ip, strconv.Itoa(port)))
}

func (s *Service) listen() {
	s.life.Lock()
	defer s.life.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	listener, err := listenAddress(s.api.IP, s.api.Port)
	if err != nil {
		logx.Errorf("listen automatic task API failed: ip=%s port=%d error=%v", s.api.IP, s.api.Port, err)
		s.api.Available = false
		s.api.ErrorCode = status_error.AutomaticTaskAPIUnavailable.Key()
		return
	}
	s.serveLocked(listener)
}

// serveLocked 在持有生命周期锁时登记监听服务和退出等待。
func (s *Service) serveLocked(listener net.Listener) {
	server := &http.Server{Handler: s.config.APIHandler(s, s.config, s.api.IP), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 8192, BaseContext: func(net.Listener) context.Context { return s.ctx }}
	s.server = server
	s.listener = listener
	s.api.Available = true
	s.api.ErrorCode = ""
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Errorf("serve automatic task API failed: error=%v", err)
			s.life.Lock()
			if s.server == server {
				s.api.Available = false
				s.api.ErrorCode = status_error.AutomaticTaskAPIUnavailable.Key()
			}
			s.life.Unlock()
		}
	}()
}

// GetAPIStatus 返回监听地址、端口及当前监听结果。
func (s *Service) GetAPIStatus() APIStatus {
	s.life.Lock()
	defer s.life.Unlock()
	return s.api
}

// SetAPIAddress 在新地址监听及持久保存成功后切换入口，失败恢复原监听。
func (s *Service) SetAPIAddress(ctx context.Context, ip string, port int) error {
	s.portGate.Lock()
	defer s.portGate.Unlock()
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.IsMulticast() || port < 1 || port > 65535 {
		return status_error.BadRequest
	}
	if ctx.Err() != nil {
		return status_error.OperationCanceled
	}
	ip = parsed.String()
	s.life.Lock()
	if s.ctx == nil || s.ctx.Err() != nil {
		s.life.Unlock()
		return status_error.OperationCanceled
	}
	old, server, oldListener := s.api, s.server, s.listener
	s.life.Unlock()
	if old.Available && old.IP == ip && old.Port == port {
		return nil
	}
	releasedOld, committed := false, false
	defer func() {
		if releasedOld && !committed {
			s.listen()
		}
	}()
	listener, err := listenAddress(ip, port)
	// 相同端口的通配地址与旧地址可能互斥；先释放，再尝试并在失败时恢复。
	if err != nil && server != nil && old.Port == port {
		_ = server.Close()
		if oldListener != nil {
			_ = oldListener.Close()
		}
		releasedOld = true
		listener, err = listenAddress(ip, port)
	}
	if err != nil {
		logx.Errorf("configure automatic task API failed: ip=%s port=%d error=%v", ip, port, err)
		return status_error.AutomaticTaskAPIUnavailable
	}
	defer func() {
		if !committed {
			_ = listener.Close()
		}
	}()
	data, _ := json.Marshal(APIStatus{IP: ip, Port: port})
	path := filepath.Join(s.config.DataDirectory, "automatic-tasks-api.json")
	if err := os.MkdirAll(s.config.DataDirectory, 0700); err != nil {
		logx.Errorf("create automatic task API settings directory failed: error=%v", err)
		return status_error.AutomaticTaskAPIUnavailable
	}
	if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
		logx.Errorf("write automatic task API settings failed: error=%v", err)
		return status_error.AutomaticTaskAPIUnavailable
	}
	defer os.Remove(path + ".tmp")
	if ctx.Err() != nil || s.ctx.Err() != nil {
		return status_error.OperationCanceled
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		logx.Errorf("replace automatic task API settings failed: error=%v", err)
		return status_error.AutomaticTaskAPIUnavailable
	}
	s.life.Lock()
	s.api = APIStatus{IP: ip, Port: port}
	s.serveLocked(listener)
	s.life.Unlock()
	committed = true
	if server != nil && !releasedOld {
		shutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = server.Shutdown(shutdown)
		cancel()
	}
	return nil
}

// SealWaker 为伙伴聚合删除持有接纳边界，返回必须释放的恢复函数。
func (s *Service) SealWaker(wakerID string) func() {
	s.gate.Lock()
	s.sealedWakers[wakerID] = true
	return func() {
		delete(s.sealedWakers, wakerID)
		s.gate.Unlock()
	}
}
