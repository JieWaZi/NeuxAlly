package skills

import (
	"context"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// 技能响应
type WakerSkillResponse struct {
	// 技能包
	Skill models.SkillPackage `json:"skill"`
	// 当前版本
	CurrentVersion models.SkillVersion `json:"currentVersion"`
	// 启用状态
	Enabled bool `json:"enabled"`
	// 影响伙伴数
	AffectedWakerCount int64 `json:"affectedWakerCount"`
}

func visibleSkill(ctx context.Context, wakerID string, skillID string) (models.SkillPackage, error) {
	if _, err := global.WakerControllerManager.Get(ctx, wakerID); err != nil {
		return models.SkillPackage{}, err
	}
	items, err := global.SkillControllerManager.ListVisible(ctx, wakerID, "")
	if err != nil {
		return models.SkillPackage{}, err
	}
	for _, item := range items {
		if item.SkillPackage.ID == skillID {
			return item.SkillPackage, nil
		}
	}
	return models.SkillPackage{}, status_error.SkillNotFound.WithParams(map[string]interface{}{"SkillID": skillID})
}

func wakerSkillResponses(
	ctx context.Context,
	items []models.SkillWithState,
) ([]WakerSkillResponse, error) {
	var publicWakerCount int64
	for _, item := range items {
		if item.SkillPackage.Scope != constants.SkillScopePrivate {
			count, err := global.WakerControllerManager.Count(ctx, "")
			if err != nil {
				return nil, err
			}
			publicWakerCount = count
			break
		}
	}
	responses := make([]WakerSkillResponse, 0, len(items))
	for _, item := range items {
		affectedWakerCount := int64(1)
		if item.SkillPackage.Scope != constants.SkillScopePrivate {
			affectedWakerCount = publicWakerCount
		}
		responses = append(responses, WakerSkillResponse{
			Skill:              item.SkillPackage,
			CurrentVersion:     item.CurrentVersion,
			Enabled:            item.Enabled,
			AffectedWakerCount: affectedWakerCount,
		})
	}
	return responses, nil
}
