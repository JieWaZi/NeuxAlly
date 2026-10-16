package authorization

import (
	"context"
	"net/url"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
)

// Device 是适配器报告的用户确认入口，不包含内部设备凭据或访问令牌。
type Device struct {
	// VerificationURL 是官方原始确认地址。
	VerificationURL string
	// UserCode 是需用户输入的一次性验证码。
	UserCode string
	// ExpiresAt 是本机授权等待期限。
	ExpiresAt int64
}

// Driver 只解释一个官方提供者的进程与配置契约。
type Driver interface {
	Binary() string
	Environment() map[string]string
	Configure(context.Context, string, string) error
	Logout(context.Context) error
	Status(context.Context, bool) (Account, error)
	Login(context.Context, []string, func(Device) error) error
}

// Config 装配实际官方边界和不包含凭据的变更通知。
type Config struct {
	// Driver 是官方进程与配置适配器。
	Driver Driver
	// Scopes 是该提供者的最小权限。
	Scopes []string
	// Changed 只用于发布状态变更，不得把会话验证码放入事件载荷。
	Changed func(Session)
}

// Authorizer 拥有唯一共享 profile 的等待和工作占用，不持久化第二份账号。
type Authorizer struct {
	// config 是提供者协议与最小权限。
	config Config
	// mu 保护会话和身份占用。
	mu sync.Mutex
	// root 是 Core 生命周期。
	root context.Context
	// stop 结束 Core 生命周期。
	stop context.CancelFunc
	// cancel 取消当前真实设备流程。
	cancel context.CancelCauseFunc
	// session 是可交付当前界面的安全摘要。
	session Session
	// usages 是当前工作对共享身份的引用。
	usages map[string]Usage
	// active 是共享身份的实际占用数。
	active int
	// deviceRunning 在真实进程退出前阻止身份变更与新工作。
	deviceRunning bool
	// workers 等待授权进程完整结束。
	workers sync.WaitGroup
}

// New 不启动后台协程，生命周期由 Open/Close 管理。
func New(config Config) *Authorizer {
	config.Scopes = slices.Clone(config.Scopes)
	return &Authorizer{config: config, usages: map[string]Usage{}, session: Session{Account: Account{Scopes: []string{}}}}
}
func (a *Authorizer) Open(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.root == nil {
		a.root, a.stop = context.WithCancel(ctx)
	}
}
func (a *Authorizer) Close() {
	a.mu.Lock()
	if a.stop != nil {
		a.stop()
	}
	if a.cancel != nil {
		a.cancel(context.Canceled)
	}
	a.mu.Unlock()
	a.workers.Wait()
}
func (a *Authorizer) Binary() string                 { return a.config.Driver.Binary() }
func (a *Authorizer) Environment() map[string]string { return a.config.Driver.Environment() }

// RequiredScopes 合并提供者及插件权限，保持稳定顺序且不跨提供者追加权限。
func (a *Authorizer) RequiredScopes(requested ...string) []string {
	result := []string{}
	for _, scope := range append(slices.Clone(a.config.Scopes), requested...) {
		if !slices.Contains(result, scope) {
			result = append(result, scope)
		}
	}
	return result
}

// ScopeGranted 委托提供者解释权限包含关系，默认使用精确匹配。
func (a *Authorizer) ScopeGranted(granted []string, required string) bool {
	if driver, ok := a.config.Driver.(interface{ ScopeGranted([]string, string) bool }); ok {
		return driver.ScopeGranted(granted, required)
	}
	return slices.Contains(granted, required)
}

func (a *Authorizer) Status(ctx context.Context, verify bool) (Account, error) {
	result, err := a.config.Driver.Status(ctx, verify)
	for _, scope := range a.config.Scopes {
		if !a.ScopeGranted(result.Scopes, scope) {
			result.Available = false
		}
	}
	return result, err
}

// AuthorizeServer 仅由确实需要远程凭据的适配器提供，令牌不能进入共享配置或快照。
func (a *Authorizer) AuthorizeServer(ctx context.Context, address string) (map[string]string, error) {
	if driver, ok := a.config.Driver.(interface {
		AuthorizeServer(context.Context, string) (map[string]string, error)
	}); ok {
		return driver.AuthorizeServer(ctx, address)
	}
	return map[string]string{}, nil
}

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
func (a *Authorizer) Usages() []Usage {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]Usage, 0, len(a.usages))
	for _, u := range a.usages {
		result = append(result, u)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ExecutionID < result[j].ExecutionID })
	return result
}
func (a *Authorizer) Configure(ctx context.Context, id, secret string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 || a.deviceRunning {
		return status_error.PluginBusy
	}
	if err := a.config.Driver.Configure(ctx, id, secret); err != nil {
		return err
	}
	a.session = Session{Account: Account{Scopes: []string{}}}
	return nil
}
func (a *Authorizer) Logout(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.active > 0 || a.deviceRunning {
		return status_error.PluginBusy
	}
	if err := a.config.Driver.Logout(ctx); err != nil {
		return err
	}
	a.session = Session{Account: Account{Scopes: []string{}}}
	return nil
}
func (a *Authorizer) Snapshot() Session { a.mu.Lock(); defer a.mu.Unlock(); return a.snapshotLocked() }
func (a *Authorizer) snapshotLocked() Session {
	s := a.session
	s.Account.Scopes = append([]string{}, s.Account.Scopes...)
	return s
}

// Login 的 HTTP 请求只等待确认入口；真实授权由 Core 根生命周期继续拥有。
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
	if err != nil || !account.Configured {
		a.mu.Unlock()
		if err != nil {
			return Session{}, err
		}
		return Session{}, status_error.PluginNotReady
	}
	work, cancel := context.WithCancelCause(a.root)
	a.cancel = cancel
	a.deviceRunning = true
	a.session = Session{ID: util.New(), State: constants.PluginAuthorizationStateWaiting, Account: account}
	id := a.session.ID
	scopes := a.RequiredScopes(requested...)
	initialized := make(chan struct{}, 1)
	finished := make(chan struct{})
	a.workers.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.workers.Done()
		defer close(finished)
		defer cancel(context.Canceled)
		timeout := time.AfterFunc(30*time.Second, func() { cancel(context.DeadlineExceeded) })
		defer timeout.Stop()
		err := a.config.Driver.Login(work, scopes, func(device Device) error {
			parsed, err := url.Parse(device.VerificationURL)
			duration := time.Until(time.UnixMilli(device.ExpiresAt))
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || duration <= 0 || duration > 30*time.Minute {
				return status_error.PluginAuthorizationFailed
			}
			a.mu.Lock()
			if a.session.ID != id || a.session.State != constants.PluginAuthorizationStateWaiting {
				a.mu.Unlock()
				return context.Canceled
			}
			a.session.VerificationURL = device.VerificationURL
			a.session.UserCode = device.UserCode
			a.session.ExpiresAt = device.ExpiresAt
			snapshot := a.snapshotLocked()
			a.mu.Unlock()
			timeout.Reset(duration)
			a.publish(snapshot)
			select {
			case initialized <- struct{}{}:
			default:
			}
			return nil
		})
		result := Account{Scopes: []string{}}
		if err == nil {
			result, err = a.Status(work, true)
			for _, scope := range scopes {
				if !a.ScopeGranted(result.Scopes, scope) {
					result.Available = false
				}
			}
			if !result.Available {
				err = status_error.PluginAuthorizationFailed
			}
		}
		a.mu.Lock()
		a.deviceRunning = false
		a.cancel = nil
		if a.session.ID == id && a.session.State == constants.PluginAuthorizationStateWaiting {
			a.session.State = constants.PluginAuthorizationStateReady
			if err != nil {
				a.session.State = constants.PluginAuthorizationStateFailed
				a.session.ErrorCode = "PluginAuthorizationFailed"
				if context.Cause(work) == context.DeadlineExceeded {
					a.session.State = constants.PluginAuthorizationStateExpired
				} else if work.Err() != nil {
					a.session.State = constants.PluginAuthorizationStateCancelled
				}
			}
			a.session.VerificationURL = ""
			a.session.UserCode = ""
			a.session.Account = result
		}
		snapshot := a.snapshotLocked()
		a.mu.Unlock()
		a.publish(snapshot)
	}()
	select {
	case <-initialized:
		return a.Snapshot(), nil
	case <-finished:
		snapshot := a.Snapshot()
		if snapshot.State == constants.PluginAuthorizationStateReady {
			return snapshot, nil
		}
		return Session{}, status_error.PluginAuthorizationFailed
	case <-ctx.Done():
		cancel(ctx.Err())
		return Session{}, ctx.Err()
	}
}

// Cancel 立即清理短时交互信息，但占用必须留到真实进程结束。
func (a *Authorizer) Cancel(id string) error {
	a.mu.Lock()
	if a.session.ID != id {
		a.mu.Unlock()
		return status_error.PluginNotFound
	}
	if a.session.State == constants.PluginAuthorizationStateWaiting {
		a.session.State = constants.PluginAuthorizationStateCancelled
		a.session.VerificationURL = ""
		a.session.UserCode = ""
		if a.cancel != nil {
			a.cancel(context.Canceled)
		}
	}
	snapshot := a.snapshotLocked()
	a.mu.Unlock()
	a.publish(snapshot)
	return nil
}
func (a *Authorizer) publish(session Session) {
	if a.config.Changed != nil {
		a.config.Changed(session)
	}
}
