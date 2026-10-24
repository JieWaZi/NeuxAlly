package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/apis/middleware"
	"github.com/JieWaZi/NeuxAlly/server/apis/uploads"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
	"os"
	"strings"
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
	location := o.Body.Location
	accepted := false
	if strings.HasPrefix(location, "upload:") {
		paths, release, err := uploads.Claim(ctx, []string{location})
		if err != nil {
			return nil, err
		}
		defer func() { release(accepted) }()
		location = paths[0]
	} else if strings.HasPrefix(middleware.AuthSubject(ctx), "web:") {
		// Web 的本机目录由用户显式输入；压缩包必须经上传，不能借用任意文件路径。
		info, err := os.Stat(location)
		if err != nil || !info.IsDir() {
			return nil, status_error.BadRequest
		}
	}
	result, err := packageplugin.Stage(stagedRoot(), location)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	accepted = true
	return result, nil
}
