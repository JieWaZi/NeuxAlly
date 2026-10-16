package plugins

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/catalog"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 准备目录插件
type PrepareCatalogPlugin struct {
	ginx.MethodPost
	// 目录条目
	Key string `in:"path" name:"key" validate:"required"`
	// 请求体
	Body PrepareCatalogPluginRequest `in:"body"`
}

// 目录准备请求
type PrepareCatalogPluginRequest struct {
	// 所属伙伴
	OwnerWakerID string `json:"ownerWakerId,omitempty"`
	// 运行确认
	TrustConfirmed bool `json:"trustConfirmed"`
}

// Path 返回目录项安装准备入口。
func (*PrepareCatalogPlugin) Path() string { return "/catalog/:key/prepare" }

// Output 核验固定目录后复用同一受管快照和资源登记用例。
func (o *PrepareCatalogPlugin) Output(ctx *gin.Context) (interface{}, error) {
	directory, err := catalog.Resolve(global.Config.PluginResourcesDirectory, o.Key)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	stage, err := packageplugin.Stage(stagedRoot(), directory)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	resource, err := prepareResource(ctx, stage, o.Body.OwnerWakerID, "", o.Body.TrustConfirmed, "catalog:"+o.Key)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return resource, nil
}
