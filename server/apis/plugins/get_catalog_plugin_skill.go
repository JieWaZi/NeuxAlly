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

// 查看目录插件技能
type GetCatalogPluginSkill struct {
	ginx.MethodGet
	// 目录条目
	Key string `in:"path" name:"key" validate:"required"`
	// 组件标识
	SkillKey string `in:"query" name:"skillKey" validate:"required"`
	// 资源路径
	FilePath string `in:"query" name:"path"`
}

// Path 返回未安装目录插件的技能查看入口。
func (*GetCatalogPluginSkill) Path() string { return "/catalog/:key/skill" }

// Output 复验分发归档并读取临时快照，不登记资源或安装关系。
func (o *GetCatalogPluginSkill) Output(ctx *gin.Context) (interface{}, error) {
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
	skill, content, err := packageplugin.ReadSkill(stage.Root, stage.Package, o.SkillKey)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	resources, file, err := packageplugin.ReadSkillResources(stage.Root, stage.Package, o.SkillKey, o.FilePath)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	version := PluginSkillVersion{Version: stage.Package.Manifest.Version, Skill: skill}
	return PluginSkillResponse{Skill: skill, Content: content, Resources: resources, File: file, Version: version, Versions: []PluginSkillVersion{version}}, nil
}
