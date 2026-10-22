package knowledge

import (
	"context"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 删除知识库请求
type DeleteLibraryInput struct {
	// 请求标识
	RequestID string `json:"requestId" validate:"required"`
	// 当前版本
	ExpectedVersion int64 `json:"expectedVersion" validate:"required,min=1"`
}

// 永久删除知识库
type DeleteLibrary struct {
	ginx.MethodDelete
	// 知识库标识
	LibraryID string `in:"path" name:"libraryId" validate:"required"`
	// 请求体
	Body DeleteLibraryInput `in:"body"`
}

// Path 返回知识库删除操作的路由。
func (o *DeleteLibrary) Path() string { return "/libraries/:libraryId" }

// Output 执行已确认的删除操作并保留原请求回执。
func (o *DeleteLibrary) Output(ctx *gin.Context) (interface{}, error) {
	if err := engineAvailable(ctx); err != nil {
		return nil, err
	}
	r, err := executeWrite(ctx, o.Body.RequestID, "library.delete", o.LibraryID, o.Body, func() (*core.Receipt, error) {
		var result *core.Receipt
		run := func() error {
			var err error
			result, err = global.KnowledgeEngine.DeleteLibrary(ctx, core.DeleteLibraryInput{RequestID: o.Body.RequestID, LibraryID: o.LibraryID, ExpectedVersion: o.Body.ExpectedVersion})
			return err
		}
		// 删除与在途发布共用策略排他锁，核心事务只负责知识数据。
		var err error
		if global.KnowledgeService != nil {
			err = global.KnowledgeService.WithPolicyWrite(run)
		} else {
			err = run()
		}
		return result, err
	})
	if err != nil {
		return nil, err
	}
	// 保留原操作锚点供重试，清理已失效的模型配置和知识成果引用。
	if err := deleteLibraryHostReferences(ctx, o.LibraryID); err != nil {
		return nil, err
	}
	return receiptDTO(r), nil
}

// deleteLibraryHostReferences 仅清理知识领域引用，普通聊天由其原生命周期拥有。
func deleteLibraryHostReferences(ctx context.Context, library string) error {
	return global.KnowledgeControllerManager.DeleteLibraryReferences(ctx, "local", "local", library)
}
