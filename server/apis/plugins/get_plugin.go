package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 插件详情
type GetPlugin struct {
	ginx.MethodGet
	// 插件标识
	PluginID string `in:"path" name:"pluginId" validate:"required"`
	// 所选版本
	VersionID string `in:"query" name:"versionId"`
	// 所属伙伴
	WakerID string `in:"query" name:"wakerId"`
}

// Path 返回插件详情入口。
func (*GetPlugin) Path() string { return "/:pluginId" }

// Output 返回不可变组成、真实安装和脱敏共享配置。
func (o *GetPlugin) Output(ctx *gin.Context) (interface{}, error) {
	resource, err := visiblePlugin(ctx, o.PluginID, o.WakerID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	states, err := global.PluginControllerManager.ReadStates(ctx, []string{resource.ID})
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	selected := o.VersionID
	if selected == "" && o.WakerID != "" {
		for _, relation := range states[resource.ID].Installations {
			if relation.WakerID == o.WakerID {
				selected = relation.VersionID
			}
		}
	}
	result, err := detailFromState(ctx, resource, states[resource.ID], selected)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
