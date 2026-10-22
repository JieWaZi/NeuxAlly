// Package knowledge 拥有知识任务与独立模型服务装配，普通聊天只作为明确请求入口。
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	bridge "github.com/JieWaZi/NeuxAlly/server/internal/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	"github.com/JieWaZi/NeuxAlly/server/internal/notification/events"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/JieWaZi/neux-knowledge/httpapi"
	"github.com/JieWaZi/neux-knowledge/rules"
	"sync"
	"time"
)

// Config 连接独立知识核心与模型服务；聊天端口仅用于用户主动的普通知识对话。
type Config struct {
	// ResolveModel 验证并冻结独立模型选择，不接纳聊天或执行。
	ResolveModel func(context.Context, models.LibraryExecutionDefaults) (models.LibraryExecutionDefaults, error)
	// Generator 为冻结选择创建纯文本模型服务，不拥有知识任务调度。
	Generator func(context.Context, models.LibraryExecutionDefaults) (core.Generator, error)

	// FreezeEmbedder 按库冻结独立嵌入配置，不能继承记忆或其他库凭据。
	FreezeEmbedder func(context.Context, string, string) (core.DocumentEmbedder, error)
	// Engine 是唯一知识内容和任务权威。
	Engine *core.Engine
	// References 保存本机连接、冻结模型选择和跨库请求回执。
	References controllers.KnowledgeController
	// Connections 只读取已配置远端及受管认证，不保存远端正文。
	Connections controllers.KnowledgeConnectionController
	// Projects 查询真实项目和目录来源。
	Projects controllers.ProjectController
	// Chats 仅核对普通聊天发起的检索与明确整理请求，不参与知识后台执行。
	Chats chat.Service
	// RemoteGuard 由独立服务器策略从持久身份恢复撤权闸门；缺省拒绝远端作业发布。
	RemoteGuard func(context.Context, json.RawMessage) (func(), error)
	// Events 复用进程内提交事件。
	Events *events.Hub
}

// Service 拥有知识任务后台等待，关闭页面不影响已接纳工作。
type Service struct {
	// closing 在同一准入锁内阻止关闭后的驱动、timer与终态恢复者。
	closing bool
	// attempts 保存每次尝试唯一的模型调用 Context 与取消句柄。
	attempts map[string]*organizeAttempt

	// config 是固定装配依赖。
	config Config
	// ctx 是Core根生命周期。
	ctx context.Context
	// cancel 结束宿主等待，不撤销核心已接纳任务。
	cancel context.CancelFunc
	// policy 串行化本机关系撤销与实际知识发布。
	policy sync.RWMutex
	// mutex 串行保护进程内作业占用。
	mutex sync.Mutex
	// running 保存本进程正在驱动的实际尝试，避免同尝试重复及吞掉下一次重试。
	running map[string]int
	// restart 保存用户显式重试后等待旧驱动退出的原任务。
	restart map[string]models.KnowledgeTaskConfiguration
	// workers 等待已启动后台任务退出。
	workers sync.WaitGroup
}

// New 只装配，不启动goroutine。
func New(config Config) *Service {
	return &Service{config: config, running: map[string]int{}, restart: map[string]models.KnowledgeTaskConfiguration{}}
}

// Start 先恢复跨库回执引用；只恢复已接纳任务的检查点，不重建任务。
func (s *Service) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	links, err := s.config.References.ListPendingOperations(ctx)
	if err != nil {
		return err
	}
	for _, link := range links {
		receipt, err := s.config.Engine.GetReceipt(ctx, link.RequestID)
		if err != nil {
			var e *core.Error
			if errors.As(err, &e) && e.Code == core.ErrNotFound {
				continue
			}
			return bridge.MapError(err)
		}
		if err := s.config.References.CompleteOperation(ctx, link.RequestID, receipt.LibraryID, receipt.ResourceID, receipt.JobID); err != nil {
			return err
		}

	}
	links, err = s.config.References.ListJobOperations(ctx)
	if err != nil {
		return err
	}
	for _, link := range links {
		job, err := s.config.Engine.GetJob(ctx, link.ResourceID)
		if err != nil {
			mapped := bridge.MapError(err)
			// 删除后保留的历史锚点不再具有可恢复任务，不能阻断整个 Core 启动。
			if errors.Is(mapped, status_error.KnowledgeNotFound) {
				continue
			}
			return mapped
		}
		if job.State == "queued" || job.State == "processing" || job.State == "cancel_requested" {
			session, err := s.config.References.GetTaskConfiguration(ctx, link.ConfigurationID)
			if err != nil {
				return err
			}
			if job.CancelRequested || session.RemoteSubject == "" || s.config.RemoteGuard != nil {
				s.drive(job.ID, session)
			}
		}
	}
	return nil
}

// Close 收敛当前等待，核心作业保留以便重启对账。
func (s *Service) Close() {
	s.mutex.Lock()
	s.closing = true
	if s.cancel != nil {
		s.cancel()
	}
	s.mutex.Unlock()
	s.workers.Wait()
}

// OrganizeCommand 是两个真实入口共用的明确整理请求。
type OrganizeCommand struct {
	// Rebuild 显式重建所选原件，省略时保留已处理来源的跳过语义与旧请求摘要。
	Rebuild bool `json:"rebuild,omitempty"`

	// RequestID 是用户请求或自动机会稳定身份。
	RequestID string
	// LibraryID 是明确目标库。
	LibraryID string
	// SourceVersionIDs 为空时只选择当前pending来源；非空精确限定。
	SourceVersionIDs []string
	// ExplicitSources 表示空集合也是已冻结的无新增，不重新选择pending。
	ExplicitSources bool
	// Configuration 是用户明确选择的独立模型配置。
	Configuration models.KnowledgeTaskConfiguration
	// Message 是实际用户维护要求，不从模型文本反推授权。
	Message string
}

// Organize 接纳并后台驱动明确范围，普通上传不调用它。
func (s *Service) Organize(ctx context.Context, in OrganizeCommand) (*core.Receipt, models.KnowledgeTaskConfiguration, error) {
	if in.Rebuild && len(in.SourceVersionIDs) == 0 {
		return nil, in.Configuration, status_error.KnowledgeInvalid
	}
	if in.RequestID == "" {
		return nil, in.Configuration, status_error.KnowledgeInvalid
	}
	if in.Configuration.ID == "" {
		in.Configuration.ID = in.RequestID
	}
	in.Configuration.KnowledgeBaseID = in.LibraryID
	in.Configuration.Backend = "local"
	in.Configuration.ConnectionID = "local"
	in.Configuration.Message = in.Message
	// 摘要固定用户意图，不能因后续pending来源或Runtime默认变化改写同键请求。
	raw, _ := json.Marshal(in)
	sum := sha256.Sum256(raw)
	link := models.KnowledgeOperationLink{RequestID: in.RequestID, Operation: "job.accept", Digest: hex.EncodeToString(sum[:]), Backend: "local", ConnectionID: "local", KnowledgeBaseID: in.LibraryID, ConfigurationID: in.Configuration.ID}
	if err := s.config.References.BeginOperation(ctx, link); err != nil {
		return nil, in.Configuration, err
	}
	if old, err := s.config.Engine.GetReceipt(ctx, in.RequestID); err == nil {
		session, err := s.config.References.GetTaskConfiguration(ctx, in.Configuration.ID)
		if err != nil {
			return nil, in.Configuration, err
		}
		if err := s.config.References.CompleteOperation(ctx, in.RequestID, old.LibraryID, old.ResourceID, old.JobID); err != nil {
			return nil, session, err
		}
		if old.ResourceID != "" {
			s.drive(old.ResourceID, session)
		}
		return old, session, nil
	} else {
		var e *core.Error
		if !errors.As(err, &e) || e.Code != core.ErrNotFound {
			return nil, in.Configuration, bridge.MapError(err)
		}
	}
	lib, err := s.config.Engine.GetLibrary(ctx, in.LibraryID)
	if err != nil {
		return nil, in.Configuration, bridge.MapError(err)
	}
	// 核心接纳暂未完成也沿用已落盘选择，不能重新解析后来变化的库或 Runtime 默认值。
	frozenConfiguration := false
	if _, err := s.config.References.GetTaskConfiguration(ctx, in.Configuration.ID); err == nil {
		in.Configuration, err = s.config.References.FreezeTaskConfiguration(ctx, in.Configuration)
		if err != nil {
			return nil, in.Configuration, err
		}
		frozenConfiguration = true
	} else if !errors.Is(err, status_error.KnowledgeNotFound) {
		return nil, in.Configuration, err
	}
	// 用户未覆盖配置时使用同一宿主持久偏好；已接纳回执的重放已在上方恢复旧配置。
	if !frozenConfiguration && in.Configuration.RuntimeID == "" && in.Configuration.AccountID == "" && in.Configuration.Model == "" {
		defaults, e := s.config.References.LibraryExecutionDefaults(ctx, in.LibraryID)
		if e != nil {
			return nil, in.Configuration, e
		}
		in.Configuration.RuntimeID = defaults.RuntimeID
		in.Configuration.AccountID = defaults.AccountID
		in.Configuration.Model = defaults.Model
	}
	if err := s.validateLibrary(lib); err != nil {
		return nil, in.Configuration, err
	}
	if !frozenConfiguration {
		if s.config.ResolveModel == nil {
			return nil, in.Configuration, status_error.KnowledgeUnsupported
		}
		selected, err := s.config.ResolveModel(ctx, models.LibraryExecutionDefaults{RuntimeID: in.Configuration.RuntimeID, AccountID: in.Configuration.AccountID, Model: in.Configuration.Model})
		if err != nil {
			return nil, in.Configuration, err
		}
		in.Configuration.RuntimeID, in.Configuration.AccountID, in.Configuration.Model = selected.RuntimeID, selected.AccountID, selected.Model
	}
	if len(in.SourceVersionIDs) == 0 && !in.ExplicitSources {
		in.SourceVersionIDs, err = s.pendingSourceHeads(ctx, in.LibraryID)
		if err != nil {
			return nil, in.Configuration, err
		}
	}

	wikiLite := lib.Kind == "general" && lib.IndexingStrategy.WikiEnabled
	bundle, err := rules.LoadOrganize(lib.Kind, lib.WikiLanguage, wikiLite)
	if err != nil {
		return nil, in.Configuration, status_error.KnowledgeUnsupported
	}
	embeddingFingerprint := ""
	if lib.IndexingStrategy.VectorEnabled {
		if s.config.FreezeEmbedder == nil {
			return nil, in.Configuration, status_error.KnowledgeUnsupported
		}
		frozen, e := s.config.FreezeEmbedder(ctx, lib.ID, lib.EmbeddingModelID)
		if e != nil {
			return nil, in.Configuration, e
		}
		embeddingFingerprint = frozen.Fingerprint
	}
	snapshot := core.PermissionSnapshot{PolicyVersion: lib.PolicyVersion, Scope: []string{}, Operations: []string{"organize"}}
	if in.Configuration.RemoteSubject != "" {
		var recovery httpapi.RecoveryIdentity
		if err := json.Unmarshal(in.Configuration.RemoteAuthorization, &recovery); err != nil {
			return nil, in.Configuration, status_error.KnowledgeForbidden
		}
		snapshot = recovery.Permission
	}
	in.Configuration, err = s.config.References.FreezeTaskConfiguration(ctx, in.Configuration)
	if err != nil {
		return nil, in.Configuration, err
	}
	s.policy.RLock()
	if err := s.validateLibrary(lib); err != nil {
		s.policy.RUnlock()
		return nil, in.Configuration, err
	}

	configDigest := core.OrganizeConfigDigest(lib, bundle.SHA256)
	receipt, err := s.config.Engine.AcceptJob(ctx, core.AcceptJobInput{Rebuild: in.Rebuild, RequestID: in.RequestID, LibraryID: in.LibraryID, ExpectedVersion: lib.Version, StructureVersion: lib.StructureVersion, SourceVersionIDs: in.SourceVersionIDs, Permission: snapshot, RulesVersion: bundle.Version, ConfigDigest: configDigest, EmbeddingFingerprint: embeddingFingerprint})
	s.policy.RUnlock()
	if err != nil {
		return nil, in.Configuration, bridge.MapError(err)
	}
	if err := s.config.References.CompleteOperation(ctx, in.RequestID, receipt.LibraryID, receipt.ResourceID, receipt.JobID); err != nil {
		return nil, in.Configuration, err
	}
	if receipt.ResourceID != "" {
		s.drive(receipt.ResourceID, in.Configuration)
	}
	return receipt, in.Configuration, nil
}

// validateLibrary 校验整理目标仍为有效通用库，模型服务不依赖项目或伙伴。
func (s *Service) validateLibrary(lib *core.Library) error {
	if lib.Archived || lib.Kind != "general" {
		return status_error.KnowledgeForbidden
	}
	return nil
}

func (s *Service) validateTarget(ctx context.Context, waker, project string, lib *core.Library) error {
	if lib.Archived || lib.Kind != "general" {
		return status_error.KnowledgeForbidden
	}
	if project != "" {
		p, err := s.config.Projects.Get(ctx, project)
		if err != nil {
			return err
		}
		if p.WakerID != waker {
			return status_error.KnowledgeForbidden
		}
	}
	return nil
}

// drive 只把核心作业交给同步模型端口，页面和普通聊天不拥有其生命周期。
func (s *Service) drive(jobID string, session models.KnowledgeTaskConfiguration) {
	s.mutex.Lock()
	if s.closing || s.ctx == nil || s.ctx.Err() != nil {
		s.mutex.Unlock()
		return
	}
	job, err := s.config.Engine.GetJob(s.ctx, jobID)
	if err != nil {
		s.mutex.Unlock()
		logx.Errorf("knowledge task lookup failed: job_id=%s error=%v", jobID, err)
		return
	}
	if running := s.running[jobID]; running != 0 {
		if running != job.Attempt {
			s.restart[jobID] = session
		}
		s.mutex.Unlock()
		return
	}
	s.running[jobID] = job.Attempt
	s.workers.Add(1)
	s.mutex.Unlock()
	go func() {
		defer s.workers.Done()
		defer s.emitTaskChanged(jobID, session.KnowledgeBaseID)
		defer func() {
			s.mutex.Lock()
			delete(s.running, jobID)
			next, restart := s.restart[jobID]
			delete(s.restart, jobID)
			s.mutex.Unlock()
			if restart {
				s.drive(jobID, next)
			}
		}()
		attempt, err := s.attemptContext(job, session)
		if err != nil {
			s.recordTaskFailure(job, err)
			return
		}
		defer s.finishAttempt(jobID, attempt)
		if job.CancelRequested {
			_, err = s.config.Engine.CancelOrganize(attempt.ctx, core.JobActionInput{RequestID: "knowledge-cancel-recovery/" + job.ID, JobID: job.ID, ExpectedVersion: job.Version})
			if err != nil {
				s.recordTaskFailure(job, err)
			}
			return
		}
		if s.config.Generator == nil {
			s.recordTaskFailure(job, status_error.KnowledgeUnsupported)
			return
		}
		generator, err := s.config.Generator(attempt.ctx, models.LibraryExecutionDefaults{RuntimeID: session.RuntimeID, AccountID: session.AccountID, Model: session.Model})
		if err != nil {
			s.recordTaskFailure(job, err)
			return
		}
		embedder := core.DocumentEmbedder{}
		if job.LibrarySnapshot != nil && (job.LibrarySnapshot.IndexingStrategy.VectorEnabled || job.LibrarySnapshot.IndexingStrategy.WikiEnabled && job.LibrarySnapshot.EmbeddingModelID != "") {
			if s.config.FreezeEmbedder == nil {
				err = status_error.KnowledgeUnsupported
			} else {
				embedder, err = s.config.FreezeEmbedder(attempt.ctx, job.LibraryID, job.LibrarySnapshot.EmbeddingModelID)
			}
			if err != nil && !job.LibrarySnapshot.IndexingStrategy.VectorEnabled {
				// Lite 的目录相关性排序可回退到已有目录；真正启用向量索引时仍须拒绝失败。
				logx.Warnf("knowledge taxonomy embedding unavailable: job_id=%s error=%v", jobID, err)
				embedder, err = core.DocumentEmbedder{}, nil
			}
		}
		if err == nil {
			_ = s.config.References.SetHostDiagnostic(attempt.ctx, job.RequestID, "active", "", "")
			_, err = s.config.Engine.RunOrganize(attempt.ctx, core.OrganizeInput{JobID: jobID, Instruction: session.Message, Embed: embedder.Embed, Reader: bridge.TextDocumentReader{}, Generator: generator, PublishGuard: s.publishGuard(session), PrepareDocument: func(ctx context.Context, source string, artifact core.ReadArtifact) (core.ReadArtifact, error) {
				// 核心在外部嵌入结束后取得同一授权闸门，避免撤权期间的迟到索引提交。
				indexEmbedder := embedder
				indexEmbedder.PublishGuard = s.publishGuard(session)
				return s.config.Engine.PrepareIndexedDocument(ctx, jobID, source, artifact, indexEmbedder)
			}, OnProgress: func(_ context.Context) { s.emitTaskChanged(jobID, session.KnowledgeBaseID) }, OnPublished: func(_ context.Context, _ *core.Receipt) { s.emitTaskChanged(jobID, session.KnowledgeBaseID) }})
		}
		if err != nil && s.ctx.Err() == nil {
			s.recordTaskFailure(job, err)
		} else if err == nil {
			_ = s.config.References.SetHostDiagnostic(context.Background(), job.RequestID, "idle", "", "")
		}
	}()
}

func (s *Service) emitTaskChanged(job, library string) {
	if s.config.Events != nil {
		_ = events.Emit(s.config.Events, "knowledge", "knowledge.changed", events.Resource{Kind: "knowledge_library", ID: library, ParentID: "local"}, map[string]any{"ref": bridge.LocalRef(library), "jobId": job})
	}
}

// recordTaskFailure 将不可恢复故障结算为核心失败，退出进程与资料删除沿各自生命周期收束。
func (s *Service) recordTaskFailure(job *core.Job, cause error) {
	if s.ctx != nil && s.ctx.Err() != nil {
		return
	}
	code, diagnostic := hostDiagnostic(cause)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	current, err := s.config.Engine.GetJob(ctx, job.ID)
	if err == nil && current.Attempt == job.Attempt && !current.CancelRequested && (current.State == "queued" || current.State == "processing") {
		_, err = s.config.Engine.FailOrganize(ctx, job.ID, job.Attempt, diagnostic)
	}
	hostState := "idle"
	if err != nil {
		hostState = "blocked"
		logx.Errorf("settle knowledge task failure failed: job_id=%s error=%v", job.ID, err)
	}
	if saveErr := s.config.References.SetHostDiagnostic(ctx, job.RequestID, hostState, code, diagnostic); saveErr != nil {
		logx.Errorf("knowledge task diagnostic failed: job_id=%s error=%v", job.ID, saveErr)
	}
	logx.Errorf("knowledge task stopped: job_id=%s error=%v", job.ID, cause)
}

// Resume 从原任务和原模型选择恢复，不扩大来源快照。
func (s *Service) Resume(ctx context.Context, job string) error { return s.resume(ctx, job, false) }

func (s *Service) resume(ctx context.Context, job string, restart bool) error {
	j, err := s.config.Engine.GetJob(ctx, job)
	if err != nil {
		return bridge.MapError(err)
	}
	origin, err := s.config.References.GetOperation(ctx, j.RequestID)
	if err != nil {
		return err
	}
	session, err := s.config.References.GetTaskConfiguration(ctx, origin.ConfigurationID)
	if err != nil {
		return err
	}
	release, err := s.publishGuard(session)(ctx)
	if err != nil {
		return err
	}
	release()
	s.mutex.Lock()
	if runningAttempt := s.running[job]; runningAttempt != 0 {
		if restart && j.Attempt != runningAttempt {
			s.restart[job] = session
		}
		s.mutex.Unlock()
		return nil
	}
	s.mutex.Unlock()
	s.drive(job, session)
	return nil
}

// currentResources 仅发现未归档通用库，不使用项目关系扩大知识范围。
func (s *Service) currentResources(ctx context.Context) ([]bridge.ResourceRef, error) {
	refs := []bridge.ResourceRef{}
	for offset := 0; s.config.Engine != nil; offset += 100 {
		rows, total, err := s.config.Engine.ListLibraries(ctx, offset, 100)
		if err != nil {
			return nil, bridge.MapError(err)
		}
		for _, lib := range rows {
			if lib.Kind == "general" && !lib.Archived {
				refs = append(refs, bridge.LocalRef(lib.ID))
			}
		}
		if int64(offset+100) >= total {
			break
		}
	}

	return refs, nil
}

// AgentBindings 冻结普通执行可见库；整理通过CLI选择当前消息范围并逐次校验。
func (s *Service) AgentBindings(ctx context.Context, chat models.Chat) []agentcapability.Binding {
	maintenance, err := s.maintenanceChatLink(ctx, chat.ID)
	if err != nil {
		return deniedKnowledgeReads(err)
	}
	if maintenance != nil {
		bindings, err := s.MaintenanceBindings(ctx, chat, bridge.ResourceRef{Backend: maintenance.Backend, ConnectionID: maintenance.ConnectionID, KnowledgeBaseID: maintenance.KnowledgeBaseID}, maintenance.ResourceID)
		if err != nil {
			return deniedKnowledgeReads(err)
		}
		return bindings
	}
	provider, err := s.freezeReadProvider(ctx)
	if err != nil {
		return deniedKnowledgeReads(err)
	}
	current := func(ctx context.Context, _ string) ([]bridge.ResourceRef, error) {
		return s.agentReadResources(ctx, provider)
	}
	refs, err := current(ctx, chat.ProjectID)
	if err != nil {
		return deniedKnowledgeReads(err)
	}
	bindings := bridge.ProviderReadBindings(provider, bridge.FreezeScope(chat.ProjectID, refs), current)
	for i := range bindings {
		call := bindings[i].Call
		bindings[i].Call = func(ctx context.Context, request agentcapability.Request) (agentcapability.Result, error) {
			s.policy.RLock()
			defer s.policy.RUnlock()
			return call(ctx, request)
		}
	}
	if chat.ID != "" && chat.WakerID != "" && chat.ActiveExecutionID != "" {
		bindings = append(bindings, s.chatOrganizeBinding(ctx, chat, refs))
		bindings = append(bindings, s.wikiBindings(ctx, chat, refs, provider, "")...)
	}
	return bindings
}

// WithPolicyWrite 串行化关系变更与正在提交的本地发布。
func (s *Service) WithPolicyWrite(run func() error) error {
	s.policy.Lock()
	defer s.policy.Unlock()
	return run()
}

// SetRemoteGuard 只由服务端组合根在启动恢复前装配，不能由请求修改。
func (s *Service) SetRemoteGuard(guard func(context.Context, json.RawMessage) (func(), error)) error {
	s.mutex.Lock()
	s.config.RemoteGuard = guard
	s.mutex.Unlock()
	if s.ctx == nil {
		return nil
	}
	links, err := s.config.References.ListJobOperations(s.ctx)
	if err != nil {
		return err
	}
	for _, link := range links {
		job, err := s.config.Engine.GetJob(s.ctx, link.ResourceID)
		if err != nil {
			mapped := bridge.MapError(err)
			// 删除库保留操作回执，但已经删除的作业没有可恢复调用。
			if errors.Is(mapped, status_error.KnowledgeNotFound) {
				continue
			}
			return mapped
		}
		if job.State != "queued" && job.State != "processing" && job.State != "cancel_requested" {
			continue
		}
		configuration, err := s.config.References.GetTaskConfiguration(s.ctx, link.ConfigurationID)
		if err != nil {
			return err
		}
		if configuration.RemoteSubject != "" {
			s.drive(job.ID, configuration)
		}
	}
	return nil
}

func (s *Service) publishGuard(session models.KnowledgeTaskConfiguration) func(context.Context) (func(), error) {
	return func(ctx context.Context) (func(), error) {
		remoteRelease := func() {}
		if session.RemoteSubject != "" {
			s.mutex.Lock()
			guard := s.config.RemoteGuard
			s.mutex.Unlock()
			if guard == nil {
				return nil, status_error.KnowledgeUnsupported
			}
			var err error
			remoteRelease, err = guard(ctx, session.RemoteAuthorization)
			if err != nil {
				return nil, err
			}
		}
		s.policy.RLock()
		lib, err := s.config.Engine.GetLibrary(ctx, session.KnowledgeBaseID)
		if err == nil {
			err = s.validateLibrary(lib)
		} else {
			err = bridge.MapError(err)
		}
		if err != nil {
			s.policy.RUnlock()
			remoteRelease()
			return nil, err
		}
		return func() { s.policy.RUnlock(); remoteRelease() }, nil
	}
}

// Retry 复核当前授权后重试原冻结任务，不读取其他pending来源。
func (s *Service) Retry(ctx context.Context, request, job string, version int64) (*core.Receipt, error) {
	j, err := s.config.Engine.GetJob(ctx, job)
	if err != nil {
		return nil, bridge.MapError(err)
	}
	link, err := s.config.References.GetOperation(ctx, j.RequestID)
	if err != nil {
		return nil, err
	}
	session, err := s.config.References.GetTaskConfiguration(ctx, link.ConfigurationID)
	if err != nil {
		return nil, err
	}
	release, err := s.publishGuard(session)(ctx)
	if err != nil {
		return nil, err
	}
	receipt, err := s.config.Engine.RetryJob(ctx, core.JobActionInput{RequestID: request, JobID: job, ExpectedVersion: version})
	release()
	if err != nil {
		return nil, bridge.MapError(err)
	}
	if err := s.resume(ctx, job, true); err != nil {
		return nil, err
	}
	return receipt, nil
}

// ResolveModel 验证知识模型偏好，不接纳任何后台生成或聊天执行。
func (s *Service) ResolveModel(ctx context.Context, selected models.LibraryExecutionDefaults) (models.LibraryExecutionDefaults, error) {
	if s.config.ResolveModel == nil {
		return selected, status_error.KnowledgeUnsupported
	}
	return s.config.ResolveModel(ctx, selected)
}
