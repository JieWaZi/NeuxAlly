package plugins

import (
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
	results, err := installationService().Install(ctx, o.PluginID, o.Body.VersionID, o.Body.ExpectedConfigurationRevision, o.Body.WakerIDs)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	response := InstallPluginResponse{PluginID: o.PluginID, Results: []PartnerInstallResult{}}
	for _, item := range results {
		response.Results = append(response.Results, PartnerInstallResult{WakerID: item.WakerID, Installed: item.Installed, ErrorCode: item.ErrorCode})
	}
	return response, nil
}
