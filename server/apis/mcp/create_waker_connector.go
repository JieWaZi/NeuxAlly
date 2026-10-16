package mcp

import (
	"context"
	"strings"

	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/util"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 创建连接器
type CreateWakerConnector struct {
	ginx.MethodPost
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 请求体
	Body ConnectorInput `in:"body"`
}

// Path 返回新建伙伴连接器接口路径。
func (c *CreateWakerConnector) Path() string { return "/:wakerId/connectors" }

// Output 新建公开连接器。
func (c *CreateWakerConnector) Output(ctx *gin.Context) (interface{}, error) {
	wakerID := strings.TrimSpace(c.WakerID)
	if _, err := global.WakerControllerManager.Get(ctx, wakerID); err != nil {
		return nil, ginx.WithStack(err)
	}
	connector, err := newPublicConnector(c.Body)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	err = global.TransactionControllerManager.Transaction(
		ctx,
		func(transactionContext context.Context) error {
			if err := global.MCPConnectorControllerManager.Create(
				transactionContext,
				connector,
			); err != nil {
				return err
			}
			return global.MCPConnectorControllerManager.UpsertWakerConnector(
				transactionContext,
				models.WakerMCPConnector{
					ID:             util.New(),
					WakerID:        wakerID,
					MCPConnectorID: connector.ID,
					Enabled:        true,
					CreatedAt:      connector.CreatedAt,
					UpdatedAt:      connector.UpdatedAt,
				},
			)
		},
	)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	item, err := visibleConnector(ctx, wakerID, connector.ID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	result, err := connectorResponse(ctx, item)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
