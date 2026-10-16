package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/JieWaZi/NeuxAlly/server/constants"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/shrewx/ginx/pkg/dbhelper"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkflowConsumerController 保存不可扩大的单回合使用者能力。
type WorkflowConsumerController interface {
	// CreateConsumer 插入签发锚点；重复身份保留首次白名单与凭据。
	CreateConsumer(context.Context, models.WorkflowConsumerLease) error
	// RevokeConsumer 只记录首次撤销，且限定真实聊天归属。
	RevokeConsumer(context.Context, string, string, int64) error
	// GetConsumer 读取工具请求绑定的真实回合。
	GetConsumer(context.Context, string) (models.WorkflowConsumerLease, error)
}

// workflowConsumerController 始终加入业务层提供的统一事务。
type workflowConsumerController struct {
	// db 是没有上层事务时使用的默认数据库连接。
	db *gorm.DB
}

// NewWorkflowConsumerController 构造单回合能力持久边界。
func NewWorkflowConsumerController(db *gorm.DB) WorkflowConsumerController {
	return &workflowConsumerController{db: db}
}

func (c *workflowConsumerController) CreateConsumer(ctx context.Context, value models.WorkflowConsumerLease) error {
	if err := controllerDB(ctx, c.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&value).Error; err != nil {
		return databaseError("create_workflow_consumer", value.ID, err)
	}
	return nil
}

func (c *workflowConsumerController) GetConsumer(ctx context.Context, id string) (models.WorkflowConsumerLease, error) {
	var value models.WorkflowConsumerLease
	err := controllerDB(ctx, c.db).Where("id = ?", id).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, status_error.WorkflowNotFound
	}
	if err != nil {
		return value, databaseError("get_workflow_consumer", id, err)
	}
	return value, nil
}

func (c *workflowConsumerController) RevokeConsumer(ctx context.Context, chatID, id string, now int64) error {
	if err := controllerDB(ctx, c.db).Model(&models.WorkflowConsumerLease{}).Where("id = ? AND chat_id = ? AND revoked_at IS NULL", id, chatID).Update("revoked_at", now).Error; err != nil {
		return databaseError("revoke_workflow_consumer", id, err)
	}
	return nil
}

// RestoreWorkflowConsumerAssociations 在 AutoMigrate 后搬移旧关联；运行时只读取新列。
// 每批沿新关联索引读取有限身份与原始 Context，不读取完整运行或扫描消息日志。
func RestoreWorkflowConsumerAssociations(ctx context.Context, db *gorm.DB) error {
	transactions := NewTransactionController(dbhelper.NewGormTransactionManager(db))
	for after := ""; ; {
		// candidate 保留原始 JSON，移除关联字段不能改变未知字段和大整数语义。
		type candidate struct {
			// ID 是稳定的批次游标。
			ID string
			// DefinitionID 是运行已冻结的定义归属。
			DefinitionID string
			// RevisionID 是运行已冻结的不可变版本。
			RevisionID string
			// Context 是搬移前的原始 JSON，同时用于条件更新。
			Context string
		}
		var candidates []candidate
		err := transactions.Transaction(ctx, func(ctx context.Context) error {
			tx := controllerDB(ctx, db)
			candidates = nil
			if err := tx.Model(&models.WorkflowRun{}).Select("id", "definition_id", "revision_id", "context").Where("consumer_lease_id = ? AND id > ?", "", after).Order("id ASC").Limit(constants.WorkflowConsumerPageSize).Find(&candidates).Error; err != nil {
				return err
			}
			ids := []string{}
			contexts := make(map[string]map[string]json.RawMessage, len(candidates))
			owners := make(map[string]string, len(candidates))
			for _, item := range candidates {
				var value map[string]json.RawMessage
				if err := json.Unmarshal([]byte(item.Context), &value); err != nil {
					return err
				}
				var owner, runID, definitionID, revisionID string
				_ = json.Unmarshal(value["consumerLeaseId"], &owner)
				_ = json.Unmarshal(value["runId"], &runID)
				_ = json.Unmarshal(value["workflowId"], &definitionID)
				_ = json.Unmarshal(value["revisionId"], &revisionID)
				if owner == "" || runID != item.ID || definitionID != item.DefinitionID || revisionID != item.RevisionID {
					continue
				}
				contexts[item.ID], owners[item.ID] = value, owner
				if !slices.Contains(ids, owner) {
					ids = append(ids, owner)
				}
			}
			if len(ids) == 0 {
				return nil
			}
			var leases []models.WorkflowConsumerLease
			if err := tx.Select("id", "chat_id", "definition_ids").Where("id IN ?", ids).Find(&leases).Error; err != nil {
				return err
			}
			byID := make(map[string]models.WorkflowConsumerLease, len(leases))
			for _, lease := range leases {
				byID[lease.ID] = lease
			}
			for _, item := range candidates {
				lease, ok := byID[owners[item.ID]]
				// 过期或已撤销租约仍拥有历史运行，但不能把不存在或越白名单的关联提升为授权。
				if !ok || lease.ChatID == "" || !slices.Contains(lease.DefinitionIDs, item.DefinitionID) {
					continue
				}
				value := contexts[item.ID]
				delete(value, "consumerLeaseId")
				encoded, err := json.Marshal(value)
				if err != nil {
					return err
				}
				updated := tx.Model(&models.WorkflowRun{}).Where("id = ? AND consumer_lease_id = ? AND context = ?", item.ID, "", item.Context).UpdateColumns(map[string]any{"consumer_lease_id": lease.ID, "context": string(encoded)})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return status_error.WorkflowStateConflict
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if len(candidates) < constants.WorkflowConsumerPageSize {
			return nil
		}
		after = candidates[len(candidates)-1].ID
	}
}
