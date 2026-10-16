package mcp

import (
	"strings"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 更新连接器
type UpdateWakerConnector struct {
	ginx.MethodPut
	// 伙伴标识
	WakerID string `in:"path" name:"wakerId" validate:"required"`
	// 连接器标识
	ConnectorID string `in:"path" name:"connectorId" validate:"required"`
	// 请求体
	Body ConnectorInput `in:"body"`
}

// Path 返回编辑伙伴连接器接口路径。
func (u *UpdateWakerConnector) Path() string {
	return "/:wakerId/connectors/:connectorId"
}

// Output 更新共享连接器。
func (u *UpdateWakerConnector) Output(ctx *gin.Context) (interface{}, error) {
	wakerID := strings.TrimSpace(u.WakerID)
	connectorID := strings.TrimSpace(u.ConnectorID)
	item, err := visibleConnector(
		ctx,
		wakerID,
		connectorID,
	)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	existing := item.Connector
	prepared, err := prepareConnector(existing, u.Body, &existing)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	if existing.Scope == constants.MCPConnectorScopeBuiltIn &&
		builtInDefinitionChanged(existing, prepared) {
		return nil, ginx.WithStack(status_error.BuiltInMCPConnectorReadOnly)
	}
	now := time.Now().UnixMilli()
	currentDigest := trustDigest(prepared)
	if existing.Scope == constants.MCPConnectorScopeBuiltIn {
		prepared.TrustedConfigDigest = currentDigest
		prepared.TrustedAt = existing.TrustedAt
	} else if currentDigest != existing.TrustedConfigDigest {
		if !u.Body.TrustConfirmed {
			return nil, ginx.WithStack(status_error.MCPConnectorTrustRequired)
		}
		prepared.TrustedConfigDigest = currentDigest
		prepared.TrustedAt = now
	}
	prepared.UpdatedAt = now
	if connectorConfigurationChanged(existing, prepared) {
		clearTestSnapshot(&prepared)
	}
	if err := global.MCPConnectorControllerManager.Update(ctx, prepared); err != nil {
		return nil, ginx.WithStack(err)
	}
	updated, err := visibleConnector(ctx, wakerID, connectorID)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	result, err := connectorResponse(ctx, updated)
	if err != nil {
		return nil, ginx.WithStack(err)
	}
	return result, nil
}
