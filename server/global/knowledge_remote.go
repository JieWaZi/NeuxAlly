package global

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	knowledgeservice "github.com/JieWaZi/NeuxAlly/server/services/knowledge"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/JieWaZi/neux-knowledge/httpapi"
)

// knowledgeServerProfile 只接受部署者固定配置，不暴露账号、路径或管理令牌。
type knowledgeServerProfile struct {
	ID        string `json:"id"`        // 客户端选择的稳定配置标识。
	Name      string `json:"name"`      // 客户端可见名称。
	RuntimeID string `json:"runtimeId"` // 服务端执行 Runtime。
	AccountID string `json:"accountId"` // 服务端受管账号引用。
	Model     string `json:"model"`     // 服务端模型。
}

// knowledgeServerConfiguration 是可选的独立知识监听配置，桌面控制面仍只监听回环。
type knowledgeServerConfiguration struct {
	Listen   string                   `json:"listen"`   // 独立知识服务监听地址。
	TLSCert  string                   `json:"tlsCert"`  // 可选 TLS 证书文件。
	TLSKey   string                   `json:"tlsKey"`   // 可选 TLS 私钥文件。
	Security httpapi.SecurityConfig   `json:"security"` // 逐库凭据与实际来源白名单。
	Profiles []knowledgeServerProfile `json:"profiles"` // 服务端固定执行配置。
}

// knowledgeRemoteHost 与本机共用唯一知识引擎和执行服务，仅开放知识 HTTP 合同。
type knowledgeRemoteHost struct {
	server   *http.Server                 // 独立 HTTP 生命周期，不挂载桌面 API。
	security *httpapi.PolicyStore         // 在途发布与配置撤销共享的权威策略。
	config   knowledgeServerConfiguration // 启动时固定的执行配置与监听参数。
	path     string                       // 仅宿主使用的配置文件路径。
}

var knowledgeRemote *knowledgeRemoteHost

func readKnowledgeServerConfig(path string) (knowledgeServerConfiguration, error) {
	var cfg knowledgeServerConfiguration
	f, err := os.Open(path)
	if err != nil {
		return cfg, status_error.CoreConfigurationInvalid
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil {
		return cfg, status_error.CoreConfigurationInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF || len(cfg.Security.Tokens) == 0 || (cfg.TLSCert == "") != (cfg.TLSKey == "") {
		return cfg, status_error.CoreConfigurationInvalid
	}
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:8787"
	}
	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return cfg, status_error.CoreConfigurationInvalid
	}
	seen := map[string]bool{}
	for _, p := range cfg.Profiles {
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Name) == "" || p.RuntimeID == "" || seen[p.ID] {
			return cfg, status_error.CoreConfigurationInvalid
		}
		seen[p.ID] = true
	}
	if _, err := httpapi.NewPolicyStore(cfg.Security); err != nil {
		return cfg, status_error.CoreConfigurationInvalid
	}
	return cfg, nil
}

func newKnowledgeRemoteHost(engine *core.Engine, service *knowledgeservice.Service, cfg knowledgeServerConfiguration) (*knowledgeRemoteHost, error) {
	security, err := httpapi.NewPolicyStore(cfg.Security)
	if err != nil {
		return nil, status_error.CoreConfigurationInvalid
	}
	options := httpapi.Options{Security: security}
	if len(cfg.Profiles) > 0 {
		profiles := knowledgeservice.ProfileMap{}
		for _, p := range cfg.Profiles {
			profiles[p.ID] = models.KnowledgeTaskConfiguration{RuntimeID: p.RuntimeID, AccountID: p.AccountID, Model: p.Model}
			options.Profiles = append(options.Profiles, httpapi.Profile{ID: p.ID, Name: p.Name})
		}
		adapter, err := knowledgeservice.NewRemoteAdapter(service, profiles, security)
		if err != nil {
			return nil, err
		}
		options.Execution = adapter
	}
	return &knowledgeRemoteHost{security: security, config: cfg, server: &http.Server{Handler: httpapi.New(engine, options), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}}, nil
}

func loadKnowledgeRemote(ctx context.Context) error {
	path := strings.TrimSpace(os.Getenv("ALLY_KNOWLEDGE_SERVER_CONFIG"))
	if path == "" {
		return nil
	}
	cfg, err := readKnowledgeServerConfig(path)
	if err != nil {
		return err
	}
	// 提前验证 TLS，避免启动成功后才发现监听端不可用。
	if cfg.TLSCert != "" {
		if _, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey); err != nil {
			return status_error.CoreConfigurationInvalid
		}
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return status_error.CoreConfigurationInvalid
	}
	host, err := newKnowledgeRemoteHost(KnowledgeEngine, KnowledgeService, cfg)
	if err != nil {
		_ = listener.Close()
		return err
	}
	host.path = path
	host.server.BaseContext = func(net.Listener) context.Context { return ctx }
	knowledgeRemote = host
	go func() {
		var err error
		if cfg.TLSCert != "" {
			err = host.server.ServeTLS(listener, cfg.TLSCert, cfg.TLSKey)
		} else {
			err = host.server.Serve(listener)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Errorf("knowledge remote listener stopped: %v", err)
		}
	}()
	return nil
}

// ReloadKnowledgeRemotePolicy 仅由本机认证管理入口调用，撤销与发布使用同一锁。
func ReloadKnowledgeRemotePolicy() error {
	host := knowledgeRemote
	if host == nil {
		return status_error.KnowledgeUnsupported
	}
	cfg, err := readKnowledgeServerConfig(host.path)
	if err != nil {
		return err
	}
	if cfg.Listen != host.config.Listen || cfg.TLSCert != host.config.TLSCert || cfg.TLSKey != host.config.TLSKey || !reflect.DeepEqual(cfg.Profiles, host.config.Profiles) {
		return status_error.CoreConfigurationInvalid
	}
	if err := host.security.Replace(cfg.Security); err != nil {
		return status_error.CoreConfigurationInvalid
	}
	return nil
}

func closeKnowledgeRemote() error {
	if knowledgeRemote == nil {
		return nil
	}
	host := knowledgeRemote
	knowledgeRemote = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := host.server.Shutdown(ctx)
	if err != nil {
		_ = host.server.Close()
	}
	return err
}
