package knowledge_server

import (
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 重载知识服务权限
type ReloadPolicy struct {
	ginx.MethodPost // 管理操作使用POST。
}

// 权限重载结果
type ReloadResult struct {
	Reloaded bool `json:"reloaded"` // 已生效。
}

func (o *ReloadPolicy) Path() string { return "/reload-policy" }
func (o *ReloadPolicy) Output(ctx *gin.Context) (interface{}, error) {
	if err := global.ReloadKnowledgeRemotePolicy(); err != nil {
		return nil, err
	}
	return ReloadResult{Reloaded: true}, nil
}
