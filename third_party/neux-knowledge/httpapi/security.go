package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/JieWaZi/neux-knowledge/core"
	"net"
	"net/netip"
	"strings"
	"sync"
)

// LibraryGrant 为一个明确的知识库授权；零值拒绝访问。
type LibraryGrant struct {
	Operations    []string // 明确允许的 read、write、organize、maintain、purge 操作；永久清理单独授权。
	Scope         []string // 服务器已验证的真实项目范围。
	PolicyVersion int64    // 必须与库当前授权代次一致。
}

func credentialHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// RestoreGuard 由受信宿主持久身份恢复发布闸门，原凭据撤销或范围变化时拒绝发布。
func (s *PolicyStore) RestoreGuard(e *core.Engine, id RecoveryIdentity) func(context.Context) (func(), error) {
	id.Permission.Scope = append([]string(nil), id.Permission.Scope...)
	id.Permission.Operations = append([]string(nil), id.Permission.Operations...)
	return func(ctx context.Context) (func(), error) {
		if err := ctx.Err(); err != nil {
			return nil, &APIError{Code: "cancelled", Status: 499}
		}
		if s == nil || e == nil || id.Subject == "" || id.LibraryID == "" || len(id.CredentialSHA256) != 64 || (id.Operation != "organize" && id.Operation != "maintain") {
			return nil, &APIError{Code: "forbidden", Status: 403}
		}
		s.mu.RLock()
		fail := func() (func(), error) { s.mu.RUnlock(); return nil, &APIError{Code: "policy_changed", Status: 403} }
		var principal Principal
		found := false
		for token, p := range s.config.Tokens {
			if credentialHash(token) == id.CredentialSHA256 && p.Subject == id.Subject {
				principal = p
				found = true
				break
			}
		}
		if !found {
			return fail()
		}
		g := principal.Libraries[id.LibraryID]
		if !contains(g.Operations, id.Operation) || g.PolicyVersion != id.Permission.PolicyVersion || !sameStrings(g.Scope, id.Permission.Scope) || !sameStrings(g.Operations, id.Permission.Operations) {
			return fail()
		}
		if s.config.WriteIPPolicy.Enabled {
			ip, ok := clientIP(id.RemoteAddr, id.ForwardedFor, s.config.WriteIPPolicy)
			if !ok || !inPrefixes(ip, s.config.WriteIPPolicy.Allowed) {
				return fail()
			}
		}
		l, err := e.GetLibrary(ctx, id.LibraryID)
		if err != nil {
			s.mu.RUnlock()
			return nil, err
		}
		if l.PolicyVersion != g.PolicyVersion {
			return fail()
		}
		return s.mu.RUnlock, nil
	}
}

// Principal 是服务端配置的主体和库范围，不从 HTTP 正文读取。
type Principal struct {
	// ActivityActor 可由服务端明确标记自动触发身份，省略时采用已认证 Subject。
	ActivityActor core.ActivityActor
	Subject       string                  // 稳定认证主体。
	Libraries     map[string]LibraryGrant // 逐库授权，不接受隐式通配符。
	CreateLibrary bool                    // 是否单独允许创建新库；创建不自动扩展已有授权。
}

// IPPolicy 约束所有远端写入口及在途生成发布。
type IPPolicy struct {
	Enabled        bool           // 开启且 Allowed 为空时拒绝全部写入。
	Allowed        []netip.Prefix // 实际客户端来源地址白名单。
	TrustedProxies []netip.Prefix // 明确可信的代理；只有可信 TCP 对端可提供转发链。
}

// SecurityConfig 必须通过 Replace 原子替换；调用者可在受控配置服务中持久保存。
type SecurityConfig struct {
	Tokens        map[string]Principal // 独立远端凭据，不复用宿主管理令牌。
	WriteIPPolicy IPPolicy             // 当前写入来源策略。
}

// PolicyStore 用同一读写锁串行化动态撤销与实际发布提交。
type PolicyStore struct {
	mu     sync.RWMutex   // 发布持读锁，策略变更持写锁。
	config SecurityConfig // 私有深拷贝，避免外部可变引用穿透锁。
}

func NewPolicyStore(c SecurityConfig) (*PolicyStore, error) {
	s := &PolicyStore{}
	if err := s.Replace(c); err != nil {
		return nil, err
	}
	return s, nil
}

// Replace 返回后，所有此后取得的闸门都使用新策略；先取得的发布已完成。
func (s *PolicyStore) Replace(c SecurityConfig) error {
	clone := SecurityConfig{Tokens: map[string]Principal{}, WriteIPPolicy: c.WriteIPPolicy}
	clone.WriteIPPolicy.Allowed = append([]netip.Prefix(nil), c.WriteIPPolicy.Allowed...)
	clone.WriteIPPolicy.TrustedProxies = append([]netip.Prefix(nil), c.WriteIPPolicy.TrustedProxies...)
	for _, p := range append(append([]netip.Prefix(nil), clone.WriteIPPolicy.Allowed...), clone.WriteIPPolicy.TrustedProxies...) {
		if !p.IsValid() {
			return errors.New("invalid IP prefix")
		}
	}
	for token, p := range c.Tokens {
		if strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \r\n\t") || strings.TrimSpace(p.Subject) == "" {
			return errors.New("invalid credential")
		}
		q := Principal{ActivityActor: p.ActivityActor, Subject: p.Subject, CreateLibrary: p.CreateLibrary, Libraries: map[string]LibraryGrant{}}
		for id, g := range p.Libraries {
			if id == "" || g.PolicyVersion < 1 {
				return errors.New("invalid library grant")
			}
			g.Operations = append([]string(nil), g.Operations...)
			g.Scope = append([]string(nil), g.Scope...)
			for _, op := range g.Operations {
				if op != "read" && op != "write" && op != "organize" && op != "maintain" && op != "purge" {
					return errors.New("invalid operation")
				}
			}
			q.Libraries[id] = g
		}
		clone.Tokens[token] = q
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = clone
	return nil
}
func contains(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}
func inPrefixes(a netip.Addr, ps []netip.Prefix) bool {
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
func clientIP(remote, forwarded string, p IPPolicy) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return netip.Addr{}, false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	peer = peer.Unmap()
	if !inPrefixes(peer, p.TrustedProxies) {
		return peer, true
	}
	if forwarded == "" {
		return netip.Addr{}, false
	}
	parts := strings.Split(forwarded, ",")
	if len(parts) > 32 {
		return netip.Addr{}, false
	}
	ips := make([]netip.Addr, len(parts))
	for i, x := range parts {
		a, e := netip.ParseAddr(strings.TrimSpace(x))
		if e != nil {
			return netip.Addr{}, false
		}
		ips[i] = a.Unmap()
	}
	for i := len(ips) - 1; i >= 0; i-- {
		if !inPrefixes(ips[i], p.TrustedProxies) {
			return ips[i], true
		}
	}
	return netip.Addr{}, false
}
func (s *PolicyStore) acquire(ctx context.Context, token, library, op, remote, forwarded string) (Principal, LibraryGrant, func(), error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, LibraryGrant{}, nil, &APIError{Code: "cancelled", Status: 499}
	}
	if s == nil {
		return Principal{}, LibraryGrant{}, nil, &APIError{Code: "unauthenticated", Status: 401}
	}
	s.mu.RLock()
	fail := func(code string, status int) (Principal, LibraryGrant, func(), error) {
		s.mu.RUnlock()
		return Principal{}, LibraryGrant{}, nil, &APIError{Code: code, Status: status}
	}
	p, ok := s.config.Tokens[token]
	if !ok {
		return fail("unauthenticated", 401)
	}
	g := p.Libraries[library]
	if op == "create" {
		if !p.CreateLibrary {
			return fail("forbidden", 403)
		}
	} else if op != "list" && !contains(g.Operations, op) {
		return fail("forbidden", 403)
	}
	if op != "read" && op != "list" && s.config.WriteIPPolicy.Enabled {
		ip, ok := clientIP(remote, forwarded, s.config.WriteIPPolicy)
		if !ok || !inPrefixes(ip, s.config.WriteIPPolicy.Allowed) {
			return fail("write_ip_forbidden", 403)
		}
	}
	return p, g, s.mu.RUnlock, nil
}
