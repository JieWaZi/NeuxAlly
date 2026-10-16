package skills

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 获取技能
type GetWakerSkill struct {
	ginx.MethodGet
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 技能标识
	SkillID string `in:"path" name:"skillId" validate:"required"`
}

// Path 返回技能详情接口路径。
func (g *GetWakerSkill) Path() string { return "/:wakerId/skills/:skillId" }

// Output 返回技能详情。
func (g *GetWakerSkill) Output(ctx *gin.Context) (interface{}, error) {
	if _, err := visibleSkill(ctx, g.WakerID, g.SkillID); err != nil {
		return nil, ginx.WithStack(err)
	}
	items, err := global.SkillControllerManager.ListVisible(ctx, g.WakerID, "")
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	for _, item := range items {
		if item.SkillPackage.ID == g.SkillID {
			responses, responseErr := wakerSkillResponses(ctx, []models.SkillWithState{item})
			if responseErr != nil {
				return nil, ginx.WithStack(responseErr)
			}
			return responses[0], nil
		}
	}
	return nil, ginx.WithStack(status_error.SkillNotFound)
}
