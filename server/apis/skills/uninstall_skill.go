package skills

import (
	"context"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	managedskills "github.com/JieWaZi/NeuxAlly/server/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 卸载技能
type UninstallSkill struct {
	ginx.MethodDelete
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 技能标识
	SkillID string `in:"path" name:"skillId" validate:"required"`
}

// Path 返回技能卸载接口路径。
func (u *UninstallSkill) Path() string { return "/:wakerId/skills/:skillId" }

// Output 删除技能数据库事实和全部权威版本文件。
func (u *UninstallSkill) Output(ctx *gin.Context) (interface{}, error) {
	skill, err := visibleSkill(ctx, u.WakerID, u.SkillID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if skill.BuiltIn {
		return nil, ginx.WithStack(status_error.BuiltInSkillCannotUninstall)
	}
	versions, err := global.SkillControllerManager.ListVersions(ctx, skill.ID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	err = global.TransactionControllerManager.Transaction(
		ctx,
		func(transactionContext context.Context) error {
			return global.SkillControllerManager.DeletePackage(transactionContext, skill.ID)
		},
	)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	// Runtime 使用工作区软链接时，删除权威目录会让正在启动的调用明确失败；
	// 已经读取技能内容的 Agent 调用由 Runtime 自身完成收尾。
	for _, version := range versions {
		if err := managedskills.RemoveVersion(global.Config.DataDirectory, version.ContentPath); err != nil {
			logx.Errorf(
				"remove uninstalled skill version failed: skill_id=%s version_id=%s error=%v",
				skill.ID,
				version.ID,
				err,
			)
		}
	}
	return ginx.CommonSuccessResponse(), nil
}
