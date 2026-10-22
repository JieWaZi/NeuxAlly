package knowledge_connections

import (
	"github.com/JieWaZi/NeuxAlly/server/apis/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 获取待确认远端操作
type ListRemoteOperations struct {
	ginx.MethodGet
	// 连接标识
	ConnectionID string `in:"path" name:"connectionId" validate:"required"`
	// 知识库标识
	LibraryID string `in:"path" name:"libraryId" validate:"required"`
	// 分页参数
	// 起始位置
	Offset int `in:"query" name:"offset" validate:"min=0"`
	// 每页条数
	Limit int `in:"query" name:"limit" validate:"min=0,max=100"`
}

func (o *ListRemoteOperations) Path() string { return "/:connectionId/libraries/:libraryId/operations" }
func (o *ListRemoteOperations) Output(ctx *gin.Context) (interface{}, error) {
	c, err := client(ctx, o.ConnectionID, o.LibraryID)
	if err != nil {
		return nil, err
	}
	if global.KnowledgeControllerManager == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	offset, limit := remotePagination(knowledge.Pagination{Offset: o.Offset, Limit: o.Limit})
	rows, total, err := global.KnowledgeControllerManager.ListRemoteOperations(ctx, c.connection.Backend, c.connection.ID, o.LibraryID, offset, limit)
	if err != nil {
		return nil, err
	}
	items := []RemoteOperation{}
	for _, v := range rows {
		items = append(items, RemoteOperation{Ref: remoteRef(c, v.KnowledgeBaseID), RequestID: v.RequestID, Operation: v.Operation, ReceiptConfirmed: v.ReceiptConfirmed, Rejected: v.RemoteErrorCode != 0, ErrorCode: v.RemoteErrorCode, ResourceID: v.ResourceID, JobID: v.JobID})
	}
	return RemoteOperationList{Items: items, Total: total}, nil
}
