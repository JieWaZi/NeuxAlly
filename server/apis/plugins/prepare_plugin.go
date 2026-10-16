package plugins

import (
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 准备插件
type PreparePlugin struct {
	ginx.MethodPost
	// 请求体
	Body PreparePluginRequest `in:"body"`
}

// Path 返回本机插件准备入口。
func (*PreparePlugin) Path() string { return "/prepare" }

// Output 复验暂存内容后登记资源与版本，不建立伙伴安装。
func (o *PreparePlugin) Output(ctx *gin.Context) (interface{}, error) {
	stage, err := packageplugin.OpenStaged(stagedRoot(), o.Body.Token)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	result, err := prepareResource(ctx, stage, o.Body.OwnerWakerID, o.Body.PluginID, o.Body.TrustConfirmed)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
