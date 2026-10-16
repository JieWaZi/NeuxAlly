// Package feishu 通过固定版本官方 CLI 管理应用私有共享身份与设备授权。
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
)

// Account 是可显示的共享身份摘要，不保存或返回登录 token。
type Account struct {
	// Configured 表示官方 profile 已完成应用配置。
	Configured bool `json:"configured"`
	// Available 表示用户身份与所需权限当前可用。
	Available bool `json:"available"`
	// AppID 是当前应用身份。
	AppID string `json:"appId,omitempty"`
	// ID 是操作者公开标识。
	ID string `json:"id,omitempty"`
	// Name 是操作者显示名称。
	Name string `json:"name,omitempty"`
	// Scopes 是官方返回的实际权限摘要。
	Scopes []string `json:"scopes"`
}

// Session 是本机授权等待的安全投影。
type Session struct {
	// ID 是本次等待身份。
	ID string `json:"id"`
	// State 是经过验证的授权阶段。
	State constants.PluginAuthorizationState `json:"state"`
	// VerificationURL 是官方原样返回的短时确认链接。
	VerificationURL string `json:"verificationUrl,omitempty"`
	// ExpiresAt 是等待期限的 Unix 毫秒值。
	ExpiresAt int64 `json:"expiresAt"`
	// Account 是最终确认的共享身份。
	Account Account `json:"account"`
	// ErrorCode 是不含敏感输出的稳定错误标识。
	ErrorCode string `json:"errorCode,omitempty"`
}

// Usage 是一个正在使用共享身份的工作引用，不保存输入或凭据。
type Usage struct {
	// ChatID 是关联对话标识。
	ChatID string `json:"chatId"`
	// ExecutionID 是本轮执行标识。
	ExecutionID string `json:"executionId"`
	// WakerID 是工作所属伙伴。
	WakerID string `json:"wakerId"`
	// Title 是接纳时的工作标题。
	Title string `json:"title"`
}

// Config 是官方 CLI 的程序、共享 profile 和外部边界装配。
type Config struct {
	// Binary 是已校验固定版本的程序绝对路径。
	Binary string
	// ProfileDirectory 是应用私有共享配置位置。
	ProfileDirectory string
	// Scopes 是插件声明的最小权限集合。
	Scopes []string
	// Run 是可替换的外部进程边界；空值使用受控真实 CLI。
	Run func(context.Context, []string, string) ([]byte, error)
	// Changed 发布安全状态投影，不能携带设备代码或 token。
	Changed func(Session)
}

// Authorizer 拥有唯一共享 profile 的登录等待和活跃执行占用。
type Authorizer struct {
	// config 是固定程序和共享 profile 装配。
	config Config
	// mu 保护身份变更、会话与活跃占用。
	mu sync.Mutex
	// root 是 Core 生命周期。
	root context.Context
	// stop 取消 Core 生命周期。
	stop context.CancelFunc
	// cancel 取消当前外部设备等待。
	cancel context.CancelFunc
	// session 是当前安全投影。
	session Session
	// usages 保存当前进程的工作引用，不成为第二份持久化状态。
	usages map[string]Usage
	// active 是使用当前身份的活跃执行数量。
	active int
	// deviceRunning 在实际设备进程退出前持续保护共享身份。
	deviceRunning bool
	// workers 等待当前授权进程结束。
	workers sync.WaitGroup
}

// New 构造授权组件，不启动后台流程。
func New(config Config) *Authorizer {
	if config.Run == nil {
		config.Run = commandRunner(config.Binary, config.ProfileDirectory)
	}
	config.Scopes = append([]string(nil), config.Scopes...)
	return &Authorizer{config: config, usages: map[string]Usage{}, session: Session{Account: Account{Scopes: []string{}}}}
}

// Open 绑定 Core 生命周期，构造阶段不启动协程。
func (a *Authorizer) Open(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.root == nil {
		a.root, a.stop = context.WithCancel(ctx)
	}
}

// Close 取消授权等待并等待外部进程结束。
func (a *Authorizer) Close() {
	a.mu.Lock()
	if a.stop != nil {
		a.stop()
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.workers.Wait()
}

// Acquire 为一次真实执行固定共享身份，释放前拒绝身份切换。
func (a *Authorizer) Acquire(uses ...Usage) (func(), error) {
	a.mu.Lock()
	if a.deviceRunning {
		a.mu.Unlock()
		return nil, status_error.PluginBusy
	}
	a.active++
	id := util.New()
	if len(uses) > 0 {
		a.usages[id] = uses[0]
	}
	snapshot := a.snapshotLocked()
	a.mu.Unlock()
	a.publish(snapshot)
	var once sync.Once
	return func() {
		once.Do(func() {
			a.mu.Lock()
			a.active--
			delete(a.usages, id)
			snapshot := a.snapshotLocked()
			a.mu.Unlock()
			a.publish(snapshot)
		})
	}, nil
}

// Usages 返回活跃工作的独立安全副本。
func (a *Authorizer) Usages() []Usage {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]Usage, 0, len(a.usages))
	for _, item := range a.usages {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ExecutionID < result[j].ExecutionID })
	return result
}

// Environment 返回官方 CLI 的显式共享配置位置。
func (a *Authorizer) Environment() map[string]string {
	return ManagedEnvironment(a.config.ProfileDirectory, nil)
}

// Binary 返回经过装配选择的固定程序位置。
func (a *Authorizer) Binary() string { return a.config.Binary }

// Configure 通过官方非交互命令配置应用，密钥只从 stdin 交付。
func (a *Authorizer) Configure(ctx context.Context, appID, secret string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 || a.deviceRunning {
		return status_error.PluginBusy
	}
	if strings.TrimSpace(appID) == "" || secret == "" {
		return status_error.PluginNotReady
	}
	if err := os.MkdirAll(a.config.ProfileDirectory, 0o700); err != nil {
		logx.Errorf("create plugin provider profile failed: %v", err)
		return status_error.PluginStorageError
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := a.config.Run(ctx, []string{"config", "init", "--app-id", appID, "--app-secret-stdin", "--brand", "feishu", "--lang", "zh_cn"}, secret+"\n")
	if err != nil {
		return err
	}
	a.session = Session{Account: Account{Scopes: []string{}}}
	return nil
}

// Logout 通过官方命令撤销本机共享身份，不复制或直接编辑 token。
func (a *Authorizer) Logout(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 || a.deviceRunning {
		return status_error.PluginBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := a.config.Run(ctx, []string{"auth", "logout"}, "")
	if err != nil {
		return err
	}
	a.session = Session{Account: Account{Scopes: []string{}}}
	return nil
}

// Status 查询官方权威身份；verify 明确要求服务端验证而非只读缓存。
func (a *Authorizer) Status(ctx context.Context, verify bool) (Account, error) {
	result := Account{Scopes: []string{}}
	if _, err := os.Stat(filepath.Join(a.config.ProfileDirectory, "config.json")); os.IsNotExist(err) {
		return result, nil
	} else if err != nil {
		logx.Errorf("read plugin provider profile failed: %v", err)
		return result, status_error.PluginStorageError
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	args := []string{"auth", "status", "--json"}
	if verify {
		args = append(args, "--verify")
	}
	output, err := a.config.Run(ctx, args, "")
	if err != nil {
		return result, err
	}
	var status struct {
		// AppID 是官方 profile 的应用标识。
		AppID string `json:"appId"`
		// Identities 是官方区分的用户和机器人身份。
		Identities struct {
			// User 是用户委托身份的诊断。
			User struct {
				// Available 是官方判断的可用状态。
				Available bool `json:"available"`
				// Verified 是外部服务验证结果。
				Verified *bool `json:"verified"`
				// OpenID 是用户标识。
				OpenID string `json:"openId"`
				// UserName 是用户展示名。
				UserName string `json:"userName"`
				// Scope 是已授予权限。
				Scope string `json:"scope"`
			} `json:"user"`
		} `json:"identities"`
	}
	if json.Unmarshal(output, &status) != nil || status.AppID == "" {
		return result, status_error.PluginAuthorizationFailed
	}
	user := status.Identities.User
	result = Account{Configured: true, Available: user.Available, AppID: status.AppID, ID: user.OpenID, Name: user.UserName, Scopes: strings.Fields(user.Scope)}
	if result.Scopes == nil {
		result.Scopes = []string{}
	}
	if verify && (user.Verified == nil || !*user.Verified) {
		result.Available = false
	}
	granted := map[string]bool{}
	for _, scope := range result.Scopes {
		granted[scope] = true
	}
	for _, scope := range a.config.Scopes {
		if !granted[scope] {
			result.Available = false
		}
	}
	return result, nil
}

// Snapshot 返回当前安全会话投影。
func (a *Authorizer) Snapshot() Session { a.mu.Lock(); defer a.mu.Unlock(); return a.snapshotLocked() }
func (a *Authorizer) snapshotLocked() Session {
	s := a.session
	s.Account.Scopes = append([]string{}, s.Account.Scopes...)
	return s
}

// Login 创建或恢复一个设备授权，不让 HTTP 请求拥有后台等待生命周期。
func (a *Authorizer) Login(ctx context.Context, requested ...string) (Session, error) {
	a.mu.Lock()
	if a.session.State == constants.PluginAuthorizationStateWaiting {
		s := a.snapshotLocked()
		a.mu.Unlock()
		return s, nil
	}
	if a.active > 0 || a.deviceRunning {
		a.mu.Unlock()
		return Session{}, status_error.PluginBusy
	}
	if a.root == nil || a.root.Err() != nil {
		a.mu.Unlock()
		return Session{}, status_error.PluginNotReady
	}
	account, err := a.Status(ctx, false)
	if err != nil {
		a.mu.Unlock()
		return Session{}, err
	}
	if !account.Configured {
		a.mu.Unlock()
		return Session{}, status_error.PluginNotReady
	}
	args := []string{"auth", "login", "--no-wait", "--json"}
	scopes := append(append([]string{}, a.config.Scopes...), requested...)
	seenScopes := map[string]bool{}
	uniqueScopes := []string{}
	for _, scope := range scopes {
		if !seenScopes[scope] {
			seenScopes[scope] = true
			uniqueScopes = append(uniqueScopes, scope)
		}
	}
	if len(uniqueScopes) > 0 {
		args = append(args, "--scope", strings.Join(uniqueScopes, " "))
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	output, err := a.config.Run(bounded, args, "")
	cancel()
	if err != nil {
		a.mu.Unlock()
		return Session{}, err
	}
	var device struct {
		// URL 是官方原始确认链接。
		URL string `json:"verification_url"`
		// Code 是仅保存在当前进程内的设备等待凭据。
		Code string `json:"device_code"`
		// Expires 是官方设备等待秒数。
		Expires int64 `json:"expires_in"`
	}
	if json.Unmarshal(output, &device) != nil || device.Code == "" || device.Expires <= 0 || device.Expires > 1800 {
		a.mu.Unlock()
		return Session{}, status_error.PluginAuthorizationFailed
	}
	parsed, err := url.Parse(device.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		a.mu.Unlock()
		return Session{}, status_error.PluginAuthorizationFailed
	}
	waiting, stop := context.WithTimeout(a.root, time.Duration(device.Expires)*time.Second)
	a.cancel = stop
	a.deviceRunning = true
	a.session = Session{ID: util.New(), State: constants.PluginAuthorizationStateWaiting, VerificationURL: device.URL, ExpiresAt: time.Now().Add(time.Duration(device.Expires) * time.Second).UnixMilli(), Account: account}
	session := a.snapshotLocked()
	a.workers.Add(1)
	a.mu.Unlock()
	a.publish(session)
	go a.complete(waiting, stop, session.ID, device.Code)
	return session, nil
}
func (a *Authorizer) complete(ctx context.Context, cancel context.CancelFunc, id, code string) {
	defer a.workers.Done()
	defer cancel()
	output, err := a.config.Run(ctx, []string{"auth", "login", "--device-code", code, "--json"}, "")
	state := constants.PluginAuthorizationStateReady
	account := Account{Scopes: []string{}}
	var result struct {
		// Event 是官方授权完成事件。
		Event string `json:"event"`
		// Missing 是官方确认仍未授予的权限。
		Missing []string `json:"missing"`
	}
	if err == nil && (json.Unmarshal(output, &result) != nil || result.Event != "authorization_complete" || len(result.Missing) > 0) {
		err = status_error.PluginAuthorizationFailed
	}
	if err == nil {
		account, err = a.Status(ctx, true)
		if err == nil && !account.Available {
			err = status_error.PluginAuthorizationFailed
		}
	}
	if err != nil {
		state = constants.PluginAuthorizationStateFailed
		if ctx.Err() == context.DeadlineExceeded {
			state = constants.PluginAuthorizationStateExpired
		} else if ctx.Err() != nil {
			state = constants.PluginAuthorizationStateCancelled
		}
	}
	a.mu.Lock()
	a.deviceRunning = false
	a.cancel = nil
	if a.session.ID != id {
		a.mu.Unlock()
		return
	}
	// 取消可先发布到界面，身份占用必须保留到外部 runner 已真正返回。
	if a.session.State != constants.PluginAuthorizationStateWaiting {
		session := a.snapshotLocked()
		a.mu.Unlock()
		a.publish(session)
		return
	}
	a.session.State = state
	a.session.VerificationURL = ""
	a.session.Account = account
	if err != nil {
		a.session.ErrorCode = "PluginAuthorizationFailed"
	}
	session := a.snapshotLocked()
	a.mu.Unlock()
	a.publish(session)
}

// Cancel 取消指定设备等待，已经完成的登录不会被隐式退出。
func (a *Authorizer) Cancel(id string) error {
	a.mu.Lock()
	if a.session.ID != id {
		a.mu.Unlock()
		return status_error.PluginNotFound
	}
	if a.session.State == constants.PluginAuthorizationStateWaiting {
		a.session.State = constants.PluginAuthorizationStateCancelled
		a.session.VerificationURL = ""
		if a.cancel != nil {
			a.cancel()
		}
	}
	session := a.snapshotLocked()
	a.mu.Unlock()
	a.publish(session)
	return nil
}
func (a *Authorizer) publish(session Session) {
	if a.config.Changed != nil {
		a.config.Changed(session)
	}
}

// boundedOutput 限制外部程序输出，避免异常 CLI 占满 Core 内存。
type boundedOutput struct {
	// buffer 保存有界 JSON 输出。
	buffer bytes.Buffer
}

func (b *boundedOutput) Write(data []byte) (int, error) {
	if b.buffer.Len()+len(data) > 1024*1024 {
		return 0, status_error.PluginContentLimitExceeded
	}
	return b.buffer.Write(data)
}
func commandRunner(binary, profile string) func(context.Context, []string, string) ([]byte, error) {
	return func(ctx context.Context, args []string, stdin string) ([]byte, error) {
		if binary == "" {
			return nil, status_error.PluginNotReady
		}
		if _, err := os.Stat(binary); err != nil {
			if os.IsNotExist(err) {
				return nil, status_error.PluginNotReady
			}
			logx.Errorf("inspect fixed plugin provider failed: %v", err)
			return nil, status_error.PluginStorageError
		}
		command := exec.CommandContext(ctx, binary, args...)
		command.Env = make([]string, 0, len(os.Environ())+1)
		for _, value := range os.Environ() {
			if !providerEnvironment(strings.SplitN(value, "=", 2)[0]) {
				command.Env = append(command.Env, value)
			}
		}
		for key, value := range ManagedEnvironment(profile, nil) {
			if value != "" {
				command.Env = append(command.Env, key+"="+value)
			}
		}
		command.Stdin = strings.NewReader(stdin)
		command.Stderr = io.Discard
		var output boundedOutput
		command.Stdout = &output
		if err := command.Run(); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			logx.Warnf("fixed Feishu CLI command failed: operation=%s exit=%v", args[0], err)
			return nil, status_error.PluginAuthorizationFailed
		}
		return output.buffer.Bytes(), nil
	}
}
