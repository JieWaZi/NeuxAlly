package plugins

import (
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 扫描插件
type ScanPlugin struct {
	ginx.MethodPost
	// 请求体
	Body ScanPluginRequest `in:"body"`
}

// Path 返回插件预检入口。
func (*ScanPlugin) Path() string { return "/scan" }

// Output 创建绑定受管快照的预检令牌。
func (o *ScanPlugin) Output(ctx *gin.Context) (interface{}, error) {
	result, err := packageplugin.Stage(stagedRoot(), o.Body.Location)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
