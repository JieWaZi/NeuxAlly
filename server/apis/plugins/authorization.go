package plugins

import (
	"context"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/notification/events"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/feishu"
)

func pluginAuthorizer(ctx context.Context, id, waker string, selectedVersion ...string) (*feishu.Authorizer, packageplugin.Extension, error) {
	resource, err := visiblePlugin(ctx, id, waker)
	if err != nil {
		return nil, packageplugin.Extension{}, err
	}
	versions, err := global.PluginControllerManager.Versions(ctx, resource.ID)
	if err != nil {
		return nil, packageplugin.Extension{}, err
	}
	if len(versions) == 0 {
		return nil, packageplugin.Extension{}, status_error.PluginNotFound
	}
	version := versions[0]
	if len(selectedVersion) > 0 && selectedVersion[0] != "" {
		found := false
		for _, candidate := range versions {
			if candidate.ID == selectedVersion[0] {
				version = candidate
				found = true
				break
			}
		}
		if !found {
			return nil, packageplugin.Extension{}, status_error.PluginVersionConflict
		}
	}
	extension, err := packageplugin.ClientExtension(version.Package)
	if err != nil {
		return nil, extension, err
	}
	if extension.Provider != constants.PluginProviderFeishu || global.FeishuAuthorization == nil {
		return nil, extension, status_error.PluginNotReady
	}
	return global.FeishuAuthorization, extension, nil
}
func publishProviderChange() {
	if global.EventHub != nil {
		_ = events.Emit(global.EventHub, constants.PluginEventTopic, "plugins.changed", events.Resource{Kind: "global"}, map[string]string{"provider": constants.PluginProviderFeishu})
	}
}
