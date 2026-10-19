// Package plugins 提供本机插件库、版本、共享配置与伙伴安装接口。
package plugins

import (
	"context"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"github.com/JieWaZi/NeuxAlly/server/services/plugininstallation"
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
	result.Account, err = installationService().Summary(version, config)
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
	return plugininstallation.New(plugininstallation.Config{Contents: global.PluginContents, DataDirectory: global.Config.DataDirectory, Authorizers: global.PluginAuthorizations, Bridges: global.PluginBridges, BridgeEnvironment: global.PluginBridgeEnvironment})
}

func listPluginResources(ctx context.Context, waker, query string, page, size int, installedOnly bool) (ListPluginsResponse, error) {
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
		resources, total, err = global.PluginControllerManager.List(txctx, "", waker, query, (page-1)*size, size, installedOnly)
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

// acquirePluginPresentation 只固定文件读取的占用，不扫描完整包；执行入口仍独立复验。
func acquirePluginPresentation(version models.PluginVersion) (string, func(), error) {
	// README 是有界短读；持有同一内容锁阻止回收，不在释放时触发全库扫描。
	release := func() {}
	if global.PluginContents != nil {
		release = global.PluginContents.Guard()
	}
	source, err := packageplugin.ContentPath(global.Config.DataDirectory, version.ContentPath)
	if err != nil {
		release()
		return "", nil, err
	}
	return source, release, nil
}

// acquirePluginReadme 为原文下载复验固定快照，并保留文件占用到读取结束。
func acquirePluginReadme(version models.PluginVersion) (string, func(), error) {
	source, release, err := acquirePluginPresentation(version)
	if err != nil {
		return "", nil, err
	}
	if err = packageplugin.VerifyContent(source, version.Digest); err != nil {
		release()
		return "", nil, err
	}
	return source, release, nil
}
