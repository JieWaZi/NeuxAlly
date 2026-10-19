package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
	"os"
	"path/filepath"
)

// 查看插件原文
type GetPluginReadme struct {
	ginx.MethodGet
	// 插件标识
	PluginID string `in:"path" name:"pluginId" validate:"required"`
	// 版本标识
	VersionID string `in:"query" name:"versionId" validate:"required"`
	// 所属伙伴
	WakerID string `in:"query" name:"wakerId"`
}

// Path 返回固定版本原文入口。
func (*GetPluginReadme) Path() string { return "/:pluginId/readme" }

// Output 复验完整快照后返回原文，正文不执行HTML或脚本。
func (o *GetPluginReadme) Output(ctx *gin.Context) (interface{}, error) {
	resource, err := visiblePlugin(ctx, o.PluginID, o.WakerID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	version, err := global.PluginControllerManager.Version(ctx, o.VersionID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if version.PluginID != resource.ID {
		return nil, ginx.WithStack(status_error.PluginVersionConflict)
	}
	source, err := packageplugin.ContentPath(global.Config.DataDirectory, version.ContentPath)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if global.PluginContents != nil {
		release, err := global.PluginContents.Acquire(source)
		if err != nil {
			return nil, ginx.WithStack(err)
		}
		defer release()
	}
	if err := packageplugin.VerifyContent(source, version.Digest); err != nil {
		return nil, ginx.WithStack(err)
	}
	root, err := packageplugin.PackageRoot(source, version.Package)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if version.Package.ReadmePath != "README.md" && version.Package.ReadmePath != "readme.md" {
		return nil, ginx.WithStack(status_error.PluginContentInvalid)
	}
	// 快照校验已拒绝越界链接，路径只来自解析器的固定文件名。
	content, err := os.ReadFile(filepath.Join(root, version.Package.ReadmePath))
	if err != nil {
		return nil, ginx.WithStack(status_error.PluginStorageError)
	}
	return PluginReadmeResponse{Content: string(content)}, nil
}

// 插件原文响应
type PluginReadmeResponse struct {
	// 完整原文
	Content string `json:"content"`
}
