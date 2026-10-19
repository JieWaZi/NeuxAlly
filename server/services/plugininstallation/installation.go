// Package plugininstallation 为界面与当前 Execution 的 allycli 复用受管插件安装用例。
package plugininstallation

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcommand"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/commandbridge"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"os"
	"path/filepath"
	"time"
)

// Config 装配共享安装的持久化与私有内容边界。
type Config struct {
	// Commands 保存Execution范围内幂等回执。
	Commands agentcommand.Store
	// Store 是唯一插件资源事实。
	Store controllers.PluginController
	// Wakers 核验安装所属伙伴。
	Wakers controllers.WakerController
	// Transactions 管理短数据库事务。
	Transactions controllers.TransactionController
	// Contents 保护执行和安装的内容占用。
	Contents *packageplugin.ContentStore
	// DataDirectory 是应用私有数据根。
	DataDirectory string
	// CatalogSource 是HTTP或本地目录清单。
	CatalogSource string
	// Bridges 复用声明驱动的共享身份。
	Bridges *commandbridge.Registry
	// BridgeEnvironment 固定受信程序资产环境。
	BridgeEnvironment []string
	// Authorizers 复用旧版受保护身份。
	Authorizers map[string]*authorization.Authorizer
}

// Service 不持有第二份安装状态。
type Service struct {
	// config 是唯一安装依赖。
	config Config
}

// New 只装配，不启动外部工作。
func New(config Config) *Service { return &Service{config: config} }

// Visible 核验资源对伙伴可见，来源名称不能替代所有权。
func (s *Service) Visible(ctx context.Context, id, waker string) (models.Plugin, error) {
	resource, err := s.config.Store.Get(ctx, id)
	if err != nil {
		return resource, err
	}
	if resource.OwnerWakerID != "" && resource.OwnerWakerID != waker {
		return resource, status_error.PluginForbidden
	}
	return resource, nil
}

// providerSnapshotKey 限定同一安装用例中的身份核验缓存。
type providerSnapshotKey struct {
	// Provider 是已核验的身份来源。
	Provider string
}

// providerSnapshot 保存同一调用中的一次核验结果。
type providerSnapshot struct {
	// Account 是公开账号摘要。
	Account authorization.Account
	// Err 是稳定的边界错误。
	Err error
}

// Readiness 核验已信任版本、共享配置和真实账号范围。
func (s *Service) Readiness(ctx context.Context, version models.PluginVersion, config models.PluginConfiguration, verify bool) (authorization.Account, error) {
	account := authorization.Account{Scopes: []string{}}
	if version.Package.RequiresTrust() && version.TrustedAt == 0 {
		return account, status_error.MCPConnectorTrustRequired
	}
	if err := packageplugin.ValidateValues(version.Package, config.Values, secretValues(config)); err != nil {
		return account, err
	}
	extension, err := packageplugin.ClientExtension(version.Package)
	if err != nil {
		return account, err
	}
	if err := packageplugin.ValidateProviderServers(extension, version.Package); err != nil {
		return account, err
	}
	available := map[string]bool{}
	for _, skill := range version.Package.Skills {
		available[skill.Key] = true
	}
	for _, server := range version.Package.Servers {
		available[server.Key] = true
	}
	for _, key := range extension.RequiredComponents {
		if !available[key] {
			return account, status_error.PluginNotReady
		}
	}
	if version.Package.Integration.Authentication != nil {
		auth, _, err := s.Bridge(version)
		if err != nil {
			return account, err
		}
		account, err = auth.Status(ctx, verify)
		if err != nil {
			return account, err
		}
		if !account.Available {
			return account, status_error.PluginNotReady
		}
		for _, scope := range version.Package.Integration.Authentication.Scopes {
			if !auth.ScopeGranted(account.Scopes, scope) {
				return account, status_error.PluginNotReady
			}
		}
		return account, nil
	}
	if extension.Provider != "" {
		if s.config.Authorizers[extension.Provider] == nil {
			return account, status_error.PluginNotReady
		}
		if _, err := os.Stat(s.config.Authorizers[extension.Provider].Binary()); err != nil {
			return account, status_error.PluginNotReady
		}
		if snapshot, ok := ctx.Value(providerSnapshotKey{Provider: extension.Provider}).(providerSnapshot); ok && !verify {
			account, err = snapshot.Account, snapshot.Err
		} else {
			account, err = s.config.Authorizers[extension.Provider].Status(ctx, verify)
		}
		if err != nil {
			return account, err
		}
		if !account.Available {
			return account, status_error.PluginNotReady
		}
		for _, scope := range extension.Scopes {
			if !s.config.Authorizers[extension.Provider].ScopeGranted(account.Scopes, scope) {
				return account, status_error.PluginNotReady
			}
		}

	}
	return account, nil
}

// Prepare 复验来源快照并幂等登记资源和版本，不创建伙伴关系。
func (s *Service) Prepare(ctx context.Context, stage packageplugin.StagedPackage, owner, target string, trusted bool, verifiedSource ...string) (models.Plugin, string, error) {
	if stage.Package.RequiresTrust() && !trusted {
		return models.Plugin{}, "", status_error.MCPConnectorTrustRequired
	}
	if owner != "" {
		if _, err := s.config.Wakers.Get(ctx, owner); err != nil {
			return models.Plugin{}, "", err
		}
	}
	extension, err := packageplugin.ClientExtension(stage.Package)
	if err != nil {
		return models.Plugin{}, "", err
	}
	// 本地清单的 repository 只是作者声明，不能证明来源身份。
	source := "local:" + stage.Digest
	if len(verifiedSource) > 0 && verifiedSource[0] != "" {
		source = verifiedSource[0]
	}
	resource := models.Plugin{}
	if target != "" {
		resource, err = s.Visible(ctx, target, owner)
	} else {
		resource, err = s.config.Store.FindSource(ctx, owner, source)
	}
	if err != nil {
		return models.Plugin{}, "", err
	}
	if resource.ID == "" {
		name := extension.DisplayName
		if name == "" {
			name = stage.Package.Presentation.DisplayName
		}
		if name == "" {
			name = stage.Package.Manifest.Name
		}
		resource = models.Plugin{ID: util.New(), Name: name, Description: stage.Package.Manifest.Description, SourceKey: source, OwnerWakerID: owner}
	}
	if s.config.Contents != nil {
		release := s.config.Contents.Guard()
		defer release()
	}
	content, err := packageplugin.CopyContent(filepath.Join(s.config.DataDirectory, "plugins", "content"), stage.Root, stage.Digest)
	if err != nil {
		return models.Plugin{}, "", err
	}
	relative, err := filepath.Rel(s.config.DataDirectory, content)
	if err != nil {
		return models.Plugin{}, "", status_error.PluginStorageError
	}
	versionName := stage.Package.Manifest.Version
	if versionName == "" {
		versionName = stage.Digest[:12]
	}
	version := models.PluginVersion{ID: util.New(), PluginID: resource.ID, Version: versionName, ContentPath: filepath.ToSlash(relative), Digest: stage.Digest, Package: stage.Package, Provenance: map[string]any{}}
	if trusted {
		version.TrustedAt = time.Now().UnixMilli()
	}
	if data, readErr := os.ReadFile(filepath.Join(stage.Root, "provenance.json")); readErr == nil {
		if json.Unmarshal(data, &version.Provenance) != nil {
			return models.Plugin{}, "", status_error.PluginContentInvalid
		}
	}
	if version.Provenance == nil {
		return models.Plugin{}, "", status_error.PluginContentInvalid
	}
	if len(verifiedSource) > 1 {
		version.Provenance["sourceCommit"] = verifiedSource[1]
	}
	if len(verifiedSource) > 2 {
		version.Provenance["archiveSHA256"] = verifiedSource[2]
	}
	if len(verifiedSource) > 3 {
		version.Provenance["archiveURL"] = verifiedSource[3]
	}
	preparedVersionID := ""
	err = s.config.Transactions.Transaction(ctx, func(txctx context.Context) error {
		existing, err := s.config.Store.FindSource(txctx, resource.OwnerWakerID, resource.SourceKey)
		if err != nil {
			return err
		}
		if target == "" && existing.ID != "" {
			resource = existing
			version.PluginID = existing.ID
		} else if existing.ID == "" {
			if _, err := s.config.Store.Get(txctx, resource.ID); errors.Is(err, status_error.PluginNotFound) {
				if err := s.config.Store.Create(txctx, resource); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		stored, err := s.config.Store.PutVersion(txctx, version)
		preparedVersionID = stored.ID
		return err
	})
	if err != nil {
		return models.Plugin{}, "", err
	}
	return resource, preparedVersionID, nil
}

func (s *Service) PrepareVersions(ctx context.Context, versions []models.PluginVersion, config models.PluginConfiguration) (func(), error) {
	releases := []func(){}
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	prepared := false
	defer func() {
		if !prepared {
			release()
		}
	}()
	providers := map[string]bool{}
	for _, version := range versions {
		root, err := packageplugin.ContentPath(s.config.DataDirectory, version.ContentPath)
		if err != nil {
			return nil, err
		}
		if s.config.Contents != nil {
			retained, err := s.config.Contents.Acquire(root)
			if err != nil {
				return nil, err
			}
			releases = append(releases, retained)
		}
		if err := packageplugin.VerifyContent(root, version.Digest); err != nil {
			return nil, err
		}
		if version.Package.Integration.Authentication != nil {
			_, _, retained, err := s.BridgeLease(version)
			if err != nil {
				return nil, err
			}
			releases = append(releases, retained)
		}
		extension, err := packageplugin.ClientExtension(version.Package)
		if err != nil {
			return nil, err
		}
		if extension.Provider != "" {
			providers[extension.Provider] = true
		}
	}
	for provider := range providers {
		if auth := s.config.Authorizers[provider]; auth != nil {
			retained, err := auth.Acquire()
			if err != nil {
				return nil, err
			}
			releases = append(releases, retained)
			account, err := auth.Status(ctx, true)
			if err != nil {
				return nil, err
			}
			ctx = context.WithValue(ctx, providerSnapshotKey{Provider: provider}, providerSnapshot{Account: account})
		}
	}
	for _, version := range versions {
		if _, err := s.Readiness(ctx, version, config, false); err != nil {
			return nil, err
		}
	}
	prepared = true
	return release, nil
}
func secretValues(config models.PluginConfiguration) map[string]string {
	values := map[string]string{}
	for _, secret := range config.Secrets {
		if secret.Configured {
			values[secret.Name] = secret.Value
		}
	}
	return values
}

// PartnerResult 记录逐伙伴的安装事实，不包含凭据。
type PartnerResult struct {
	// WakerID 是实际安装归属。
	WakerID string `json:"wakerId"`
	// Installed 表示系统安装关系已经提交。
	Installed bool `json:"installed"`
	// ErrorCode 是单项失败的稳定错误。
	ErrorCode string `json:"errorCode,omitempty"`
}

func errorKey(err error) string {
	var coded interface{ Key() string }
	if errors.As(err, &coded) {
		return coded.Key()
	}
	return status_error.InternalServerError.Key()
}

// Install 为HTTP与Execution入口提交相同的伙伴安装事务。
func (s *Service) Install(ctx context.Context, pluginID, versionID string, expectedRevision int64, wakers []string) ([]PartnerResult, error) {
	store := s.config.Store
	resource, err := store.Get(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	version, err := store.Version(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if version.PluginID != resource.ID {
		return nil, status_error.PluginVersionConflict
	}
	config, err := store.Configuration(ctx, resource.ID)
	if err != nil {
		return nil, err
	}
	if config.Revision != expectedRevision {
		return nil, status_error.PluginConfigurationConflict
	}
	release, err := s.PrepareVersions(ctx, []models.PluginVersion{version}, config)
	if err != nil {
		return nil, err
	}
	defer release()
	result := []PartnerResult{}
	seen := map[string]bool{}
	for _, waker := range wakers {
		if seen[waker] {
			continue
		}
		seen[waker] = true
		item := PartnerResult{WakerID: waker}
		if resource.OwnerWakerID != "" && resource.OwnerWakerID != waker {
			item.ErrorCode = status_error.PluginForbidden.Key()
			result = append(result, item)
			continue
		}
		if _, err := s.config.Wakers.Get(ctx, waker); err != nil {
			item.ErrorCode = errorKey(err)
			result = append(result, item)
			continue
		}
		err := s.config.Transactions.Transaction(ctx, func(txctx context.Context) error {
			current, err := store.Configuration(txctx, resource.ID)
			if err != nil {
				return err
			}
			if current.Revision != config.Revision {
				return status_error.PluginConfigurationConflict
			}
			if err := store.Install(txctx, models.WakerPlugin{ID: util.New(), WakerID: waker, PluginID: resource.ID, VersionID: version.ID, Enabled: true}); err != nil {
				return err
			}
			return agentcommand.Commit(txctx, map[string]any{"pluginId": resource.ID, "versionId": version.ID, "installed": true, "state": "installed", "nextStep": "Available to the next execution; current execution remains unchanged."})
		})
		item.Installed = err == nil
		if err != nil {
			item.ErrorCode = errorKey(err)
		}
		result = append(result, item)
	}
	return result, nil
}

// Bridge 从已核验版本创建包内协议适配，共享身份始终由Core签发。
func (s *Service) Bridge(version models.PluginVersion) (*authorization.Authorizer, *commandbridge.Driver, error) {
	auth, driver, _, err := s.bridge(version, false)
	return auth, driver, err
}

// BridgeLease 在选定版本时立即取得唯一身份租约，防止并发版本替换。
func (s *Service) BridgeLease(version models.PluginVersion, uses ...authorization.Usage) (*authorization.Authorizer, *commandbridge.Driver, func(), error) {
	return s.bridge(version, true, uses...)
}

func (s *Service) bridge(version models.PluginVersion, acquire bool, uses ...authorization.Usage) (*authorization.Authorizer, *commandbridge.Driver, func(), error) {
	if version.TrustedAt == 0 {
		return nil, nil, nil, status_error.MCPConnectorTrustRequired
	}
	declaration := version.Package.Integration.Authentication
	if declaration == nil || declaration.Strategy != constants.PluginAuthCommandStrategy || declaration.Protocol != constants.PluginAuthProtocol || s.config.Bridges == nil {
		return nil, nil, nil, status_error.PluginNotReady
	}
	source, err := packageplugin.ContentPath(s.config.DataDirectory, version.ContentPath)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := packageplugin.VerifyContent(source, version.Digest); err != nil {
		return nil, nil, nil, err
	}
	root, err := packageplugin.PackageRoot(source, version.Package)
	if err != nil {
		return nil, nil, nil, err
	}
	command, args, err := commandbridge.ResolveEntry(root, declaration.Command, declaration.Args)
	if err != nil {
		return nil, nil, nil, err
	}
	config := commandbridge.Config{Command: command, Args: args, Directory: root, DataDirectory: filepath.Join(s.config.DataDirectory, "plugins", "data", version.PluginID), PluginID: version.PluginID, Environment: s.config.BridgeEnvironment}
	if acquire {
		return s.config.Bridges.Acquire(version.PluginID, version.Digest, config, declaration.Scopes, uses...)
	}
	auth, driver, err := s.config.Bridges.Get(version.PluginID, version.Digest, config, declaration.Scopes)
	return auth, driver, nil, err
}
