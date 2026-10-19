package plugins

import (
	"os"
	"path/filepath"

	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/catalog"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 查看目录插件
type GetCatalogPlugin struct {
	ginx.MethodGet
	// 目录条目
	Key string `in:"path" name:"key" validate:"required"`
}

// Path 返回只读目录详情。
func (*GetCatalogPlugin) Path() string { return "/catalog/:key" }

// Output 按需核验完整来源，不创建系统资源或伙伴安装。
func (o *GetCatalogPlugin) Output(ctx *gin.Context) (interface{}, error) {
	entry, err := catalog.Find(global.PluginCatalogSource(), o.Key)
	if err != nil {
		return nil, ginx.WithStack(err)
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
	entry.Package = stage.Package
	return entry, nil
}
