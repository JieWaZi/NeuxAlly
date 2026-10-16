package skills

import (
	"os"
	"path/filepath"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	internalSkills "github.com/JieWaZi/NeuxAlly/server/internal/skills"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 扫描导入
type ScanWakerSkillImport struct {
	ginx.MethodPost
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 请求体
	Body ScanSkillImportRequest `in:"body"`
}

// 扫描导入请求
type ScanSkillImportRequest struct {
	// 来源类型
	Kind constants.SkillSource `json:"kind" validate:"required"`
	// 来源位置
	Location string `json:"location" validate:"required"`
	// Git 引用
	Ref string `json:"ref"`
	// 子目录
	Subpath string `json:"subpath"`
}

// 导入批次
type SkillImportBatch struct {
	// 批次令牌
	Token string `json:"token"`
	// 候选列表
	Candidates []SkillImportCandidate `json:"candidates"`
}

// 导入候选
type SkillImportCandidate struct {
	// 标识
	ID string `json:"id"`
	// 名称
	Name string `json:"name"`
	// 描述
	Description string `json:"description"`
	// 已安装
	Installed bool `json:"installed"`
	// 已装技能
	InstalledSkillID string `json:"installedSkillId,omitempty"`
}

// Path 返回技能导入扫描接口路径。
func (s *ScanWakerSkillImport) Path() string { return "/:wakerId/skills/import/scan" }

// Output 暂存来源并返回候选，不创建任何技能权威事实。
func (s *ScanWakerSkillImport) Output(ctx *gin.Context) (interface{}, error) {
	if _, err := global.WakerControllerManager.Get(ctx, s.WakerID); err != nil {
		return nil, ginx.WithStack(err)
	}
	batch, err := internalSkills.StageImport(
		ctx,
		filepath.Join(global.Config.DataDirectory, "skills", "tmp"),
		internalSkills.Source{
			Kind: s.Body.Kind, Location: s.Body.Location,
			Ref: s.Body.Ref, Subpath: s.Body.Subpath,
		},
	)
	if err != nil {
		logx.Errorf("scan skill source failed: kind=%s waker_id=%s error=%v", s.Body.Kind, s.WakerID, err)
		return nil, ginx.WithStack(err)
	}
	visible, err := global.SkillControllerManager.ListVisible(ctx, s.WakerID, "")
	if err != nil {
		_ = os.RemoveAll(batch.Root)
		return nil, ginx.WithStack(err)
	}
	installed := make(map[string]string, len(visible))
	for _, item := range visible {
		installed[item.SkillPackage.Name] = item.SkillPackage.ID
	}
	candidates := make([]SkillImportCandidate, 0, len(batch.Packages))
	for _, candidate := range batch.Packages {
		installedSkillID := installed[candidate.Name]
		candidates = append(candidates, SkillImportCandidate{
			ID: candidate.RelativePath, Name: candidate.Name,
			Description: candidate.Description,
			Installed:   installedSkillID != "", InstalledSkillID: installedSkillID,
		})
	}
	return SkillImportBatch{Token: batch.Token, Candidates: candidates}, nil
}
