package knowledge_connections

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 读取远端操作回执
type GetRemoteReceipt struct {
	ginx.MethodGet
	// 连接标识
	ConnectionID string `in:"path" name:"connectionId" validate:"required"`
	// 知识库标识
	LibraryID string `in:"path" name:"libraryId" validate:"required"`
	// 请求标识
	RequestID string `in:"path" name:"requestId" validate:"required"`
}

func (o *GetRemoteReceipt) Path() string {
	return "/:connectionId/libraries/:libraryId/receipts/:requestId"
}
func (o *GetRemoteReceipt) Output(ctx *gin.Context) (interface{}, error) {
	c, err := client(ctx, o.ConnectionID, o.LibraryID)
	if err != nil {
		return nil, err
	}
	if c.custom == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	return reconcileRemote(ctx, c, o.RequestID)
}
