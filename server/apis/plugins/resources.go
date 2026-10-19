// Package plugins 提供本机插件库、版本、共享配置与伙伴安装接口。
package plugins

import (
	"context"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"github.com/JieWaZi/NeuxAlly/server/services/plugininstallation"
	"os"
	"path/filepath"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

func visiblePlugin(ctx context.Context, id, waker string) (models.Plugin, error) {
	resource, err := global.PluginControllerManager.Get(ctx, id)
	if err != nil {
		return resource, err
	}
	if resource.OwnerWakerID != "" && resource.OwnerWakerID != waker {
		return resource, status_error.PluginForbidden
	}
	return resource, nil
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
func readiness(ctx context.Context, version models.PluginVersion, config models.PluginConfiguration, verify bool) (authorization.Account, error) {
	account := authorization.Account{Scopes: []string{}}
	if version.Package.Integration.Authentication != nil {
		return installationService().Readiness(ctx, version, config, verify)
	}
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
	if extension.Provider != "" {
		if global.PluginAuthorizations[extension.Provider] == nil {
			return account, status_error.PluginNotReady
		}
		if _, err := os.Stat(global.PluginAuthorizations[extension.Provider].Binary()); err != nil {
			return account, status_error.PluginNotReady
		}
		if snapshot, ok := ctx.Value(providerSnapshotKey{Provider: extension.Provider}).(providerSnapshot); ok && !verify {
			account, err = snapshot.Account, snapshot.Err
		} else {
			account, err = global.PluginAuthorizations[extension.Provider].Status(ctx, verify)
		}
		if err != nil {
			return account, err
		}
		if !account.Available {
			return account, status_error.PluginNotReady
		}
		for _, scope := range extension.Scopes {
			if !global.PluginAuthorizations[extension.Provider].ScopeGranted(account.Scopes, scope) {
				return account, status_error.PluginNotReady
			}
		}

	}
	return account, nil
}
func pluginDetail(ctx context.Context, resource models.Plugin, selectedVersion ...string) (PluginDetailResponse, error) {
	result := PluginDetailResponse{Plugin: resource, Versions: []models.PluginVersion{}, Installations: []models.WakerPlugin{}, Values: map[string]string{}, Fields: []packageplugin.ConfigurationField{}, ConfiguredSecrets: []string{}, Blockers: []string{}, Account: authorization.Account{Scopes: []string{}}}
	states, err := global.PluginControllerManager.ReadStates(ctx, []string{resource.ID})
	if err != nil {
		return result, err
	}
	return detailFromState(ctx, resource, states[resource.ID], selectedVersion...)
}
func detailFromState(ctx context.Context, resource models.Plugin, state models.PluginState, selectedVersion ...string) (PluginDetailResponse, error) {
	result := PluginDetailResponse{Plugin: resource, PartnerNames: state.PartnerNames, PartnerAvatars: state.PartnerAvatars, Versions: state.Versions, Installations: state.Installations, Values: map[string]string{}, Fields: []packageplugin.ConfigurationField{}, ConfiguredSecrets: []string{}, Blockers: []string{}, Account: authorization.Account{Scopes: []string{}}}
	config := state.Configuration

	result.Values = config.Values
	result.ConfigurationRevision = config.Revision
	for _, secret := range config.Secrets {
		if secret.Configured {
			result.ConfiguredSecrets = append(result.ConfiguredSecrets, secret.Name)
		}
	}
	if len(result.Versions) == 0 {
		result.Blockers = append(result.Blockers, status_error.PluginNotReady.Key())
		return result, nil
	}
	version := result.Versions[0]
	if len(selectedVersion) > 0 && selectedVersion[0] != "" {
		found := false
		for _, candidate := range result.Versions {
			if candidate.ID == selectedVersion[0] {
				version = candidate
				found = true
				break
			}
		}
		if !found {
			return result, status_error.PluginVersionConflict
		}
	}
	result.ResolvedVersionID = version.ID
	extension, err := packageplugin.ClientExtension(version.Package)
	if err != nil {
		return result, err
	}
	result.ComponentIDs = map[string]string{}
	for _, skill := range version.Package.Skills {
		result.ComponentIDs["skill:"+skill.Key] = packageplugin.ComponentID(resource.ID, "skill:"+skill.Key)
	}
	for _, server := range version.Package.Servers {
		result.ComponentIDs["mcp:"+server.Key] = packageplugin.ComponentID(resource.ID, "mcp:"+server.Key)
	}
	result.Fields = extension.Config
	result.Account, err = readiness(ctx, version, config, false)
	result.Ready = err == nil
	if err != nil {
		result.Blockers = append(result.Blockers, errorKey(err))
	}
	return result, nil
}
func errorKey(err error) string {
	var coded interface{ Key() string }
	if errors.As(err, &coded) {
		return coded.Key()
	}
	return status_error.InternalServerError.Key()
}
func stagedRoot() string { return filepath.Join(global.Config.DataDirectory, "plugins", "staging") }
func installationService() *plugininstallation.Service {
	return plugininstallation.New(plugininstallation.Config{Store: global.PluginControllerManager, Wakers: global.WakerControllerManager, Transactions: global.TransactionControllerManager, Contents: global.PluginContents, DataDirectory: global.Config.DataDirectory, CatalogSource: global.PluginCatalogSource(), Authorizers: global.PluginAuthorizations, Bridges: global.PluginBridges, BridgeEnvironment: global.PluginBridgeEnvironment})
}
func prepareResource(ctx context.Context, stage packageplugin.StagedPackage, owner, target string, trusted bool, source ...string) (PluginDetailResponse, error) {
	resource, version, err := installationService().Prepare(ctx, stage, owner, target, trusted, source...)
	if err != nil {
		return PluginDetailResponse{}, err
	}
	return pluginDetail(ctx, resource, version)
}

// providerSnapshotKey 只在一次列表查询内复用官方身份结果。
type providerSnapshotKey struct {
	// Provider 区分同一请求中的官方身份来源。
	Provider string
}

// providerSnapshot 不跨请求缓存授权事实。
type providerSnapshot struct {
	// Account 是本次查询的官方身份摘要。
	Account authorization.Account
	// Err 是同一次边界查询的稳定失败。
	Err error
}

func listPluginResources(ctx context.Context, waker, query string, page, size int) (ListPluginsResponse, error) {
	if page < 0 || size < 0 || size > 100 {
		return ListPluginsResponse{}, status_error.BadRequest
	}
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 50
	}
	if waker != "" {
		if _, err := global.WakerControllerManager.Get(ctx, waker); err != nil {
			return ListPluginsResponse{}, err
		}
	}
	var resources []models.Plugin
	var total int64
	var states map[string]models.PluginState
	err := global.TransactionControllerManager.Transaction(ctx, func(txctx context.Context) error {
		var err error
		resources, total, err = global.PluginControllerManager.List(txctx, "", waker, query, (page-1)*size, size)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(resources))
		for _, resource := range resources {
			ids = append(ids, resource.ID)
		}
		states, err = global.PluginControllerManager.ReadStates(txctx, ids)
		return err
	})
	if err != nil {
		return ListPluginsResponse{}, err
	}

	providers := map[string]bool{}
	for _, state := range states {
		for _, version := range state.Versions {
			extension, err := packageplugin.ClientExtension(version.Package)
			if err == nil && extension.Provider != "" {
				providers[extension.Provider] = true
			}
		}
	}
	for provider := range providers {
		if auth := global.PluginAuthorizations[provider]; auth != nil {
			account, err := auth.Status(ctx, false)
			ctx = context.WithValue(ctx, providerSnapshotKey{Provider: provider}, providerSnapshot{Account: account, Err: err})
		}
	}
	result := ListPluginsResponse{Resources: []PluginDetailResponse{}, TotalSize: total, PageNum: page, PageSize: size}
	for _, resource := range resources {
		selectedVersion := ""
		if waker != "" {
			for _, relation := range states[resource.ID].Installations {
				if relation.WakerID == waker {
					selectedVersion = relation.VersionID
				}
			}
		}
		detail, err := detailFromState(ctx, resource, states[resource.ID], selectedVersion)
		if err != nil {
			return result, err
		}
		for index := range detail.Versions {
			detail.Versions[index].Package.Readme = ""
		}
		result.Resources = append(result.Resources, detail)
	}
	return result, nil
}

// acquirePluginReadme 让语言预览与原文下载复用固定快照校验和文件占用边界。
func acquirePluginReadme(version models.PluginVersion) (string, func(), error) {
	source, err := packageplugin.ContentPath(global.Config.DataDirectory, version.ContentPath)
	if err != nil {
		return "", nil, err
	}
	release := func() {}
	if global.PluginContents != nil {
		release, err = global.PluginContents.Acquire(source)
		if err != nil {
			return "", nil, err
		}
	}
	if err = packageplugin.VerifyContent(source, version.Digest); err != nil {
		release()
		return "", nil, err
	}
	return source, release, nil
}
