package mcp

import (
	"context"
	"strings"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 删除连接器
type DeleteWakerConnector struct {
	ginx.MethodDelete
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 连接器标识
	ConnectorID string `in:"path" name:"connectorId" validate:"required"`
}

// Path 返回删除伙伴连接器接口路径。
func (d *DeleteWakerConnector) Path() string {
	return "/:wakerId/connectors/:connectorId"
}

// Output 全局删除公开连接器及其凭据。
func (d *DeleteWakerConnector) Output(ctx *gin.Context) (interface{}, error) {
	item, err := visibleConnector(
		ctx,
		strings.TrimSpace(d.WakerID),
		strings.TrimSpace(d.ConnectorID),
	)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if item.Connector.Scope == constants.MCPConnectorScopeBuiltIn {
		return nil, ginx.WithStack(status_error.BuiltInMCPConnectorCannotDelete)
	}
	if err := global.TransactionControllerManager.Transaction(
		ctx,
		func(transactionContext context.Context) error {
			return global.MCPConnectorControllerManager.DeleteConnector(
				transactionContext,
				item.Connector.ID,
			)
		},
	); err != nil {
		return nil, ginx.WithStack(err)
	}
	return ginx.CommonSuccessResponse(), nil
}
