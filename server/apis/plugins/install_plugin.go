package plugins

import (
	"context"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 安装插件
type InstallPlugin struct {
	ginx.MethodPost
	// 插件标识
	PluginID string `in:"path" name:"pluginId" validate:"required"`
	// 请求体
	Body InstallPluginRequest `in:"body"`
}

// Path 返回伙伴批量安装入口。
func (*InstallPlugin) Path() string { return "/:pluginId/installations" }

// Output 先验证必要认证，再逐伙伴提交真实安装，不预建占位关系。
func (o *InstallPlugin) Output(ctx *gin.Context) (interface{}, error) {
	store := global.PluginControllerManager
	resource, err := store.Get(ctx, o.PluginID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	version, err := store.Version(ctx, o.Body.VersionID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if version.PluginID != resource.ID {
		return nil, ginx.WithStack(status_error.PluginVersionConflict)
	}
	config, err := store.Configuration(ctx, resource.ID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if config.Revision != o.Body.ExpectedConfigurationRevision {
		return nil, ginx.WithStack(status_error.PluginConfigurationConflict)
	}
	release, err := prepareInstallationVersions(ctx, []models.PluginVersion{version}, config)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	defer release()
	result := InstallPluginResponse{PluginID: resource.ID, Results: []PartnerInstallResult{}}
	seen := map[string]bool{}
	for _, waker := range o.Body.WakerIDs {
		if seen[waker] {
			continue
		}
		seen[waker] = true
		item := PartnerInstallResult{WakerID: waker}
		if resource.OwnerWakerID != "" && resource.OwnerWakerID != waker {
			item.ErrorCode = status_error.PluginForbidden.Key()
			result.Results = append(result.Results, item)
			continue
		}
		if _, err := global.WakerControllerManager.Get(ctx, waker); err != nil {
			item.ErrorCode = errorKey(err)
			result.Results = append(result.Results, item)
			continue
		}
		err := global.TransactionControllerManager.Transaction(ctx, func(txctx context.Context) error {
			current, err := store.Configuration(txctx, resource.ID)
			if err != nil {
				return err
			}
			if current.Revision != config.Revision {
				return status_error.PluginConfigurationConflict
			}
			return store.Install(txctx, models.WakerPlugin{ID: util.New(), WakerID: waker, PluginID: resource.ID, VersionID: version.ID, Enabled: true})
		})
		item.Installed = err == nil
		if err != nil {
			item.ErrorCode = errorKey(err)
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}
