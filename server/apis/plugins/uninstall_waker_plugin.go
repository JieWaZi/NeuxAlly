package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 卸载伙伴插件
type UninstallWakerPlugin struct {
	ginx.MethodDelete
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 插件标识
	PluginID string `in:"path" name:"pluginId" validate:"required"`
}

// Path 返回伙伴插件关系操作入口。
func (*UninstallWakerPlugin) Path() string { return "/:wakerId/plugins/:pluginId" }

// Output 只改变该伙伴的真实安装关系。
func (o *UninstallWakerPlugin) Output(ctx *gin.Context) (interface{}, error) {
	if _, err := global.WakerControllerManager.Get(ctx, o.WakerID); err != nil {
		return nil, ginx.WithStack(err)
	}
	if _, err := visiblePlugin(ctx, o.PluginID, o.WakerID); err != nil {
		return nil, ginx.WithStack(err)
	}
	if err := global.PluginControllerManager.Uninstall(ctx, o.WakerID, o.PluginID); err != nil {
		return nil, ginx.WithStack(err)
	}
	return ginx.CommonSuccessResponse(), nil
}
