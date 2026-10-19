package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/catalog"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
	"os"
	"path/filepath"
)

// 查看目录插件原文
type GetCatalogPluginReadme struct {
	ginx.MethodGet
	// 来源提交
	SourceCommit string `in:"query" name:"sourceCommit"`
	// 压缩摘要
	ArchiveSHA256 string `in:"query" name:"archiveSHA256"`
	// 目录条目
	Key string `in:"path" name:"key" validate:"required"`
}

// Path 返回未安装插件的固定来源原文。
func (*GetCatalogPluginReadme) Path() string { return "/catalog/:key/readme" }

// Output 只读取已核验来源，不建立系统资源或安装关系。
func (o *GetCatalogPluginReadme) Output(ctx *gin.Context) (interface{}, error) {
	entry, err := catalog.Find(global.PluginCatalogSource(), o.Key)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if (o.SourceCommit != "" && o.SourceCommit != entry.SourceCommit) || (o.ArchiveSHA256 != "" && o.ArchiveSHA256 != entry.ArchiveSHA256) {
		return nil, ginx.WithStack(status_error.PluginVersionConflict)
	}
	archive, err := catalog.ResolveEntry(global.PluginCatalogSource(), entry)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	stage, err := packageplugin.StageSource(stagedRoot(), archive, entry.PackagePath)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	defer os.RemoveAll(filepath.Join(stagedRoot(), stage.Token))
	root, err := packageplugin.PackageRoot(stage.Root, stage.Package)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if stage.Package.ReadmePath != "README.md" && stage.Package.ReadmePath != "readme.md" {
		return nil, ginx.WithStack(status_error.PluginContentInvalid)
	}
	content, err := os.ReadFile(filepath.Join(root, stage.Package.ReadmePath))
	if err != nil {
		return nil, ginx.WithStack(status_error.PluginStorageError)
	}
	return PluginReadmeResponse{Content: string(content)}, nil
}
