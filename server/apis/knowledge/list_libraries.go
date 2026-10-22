package knowledge

import (
	"context"
	"github.com/JieWaZi/NeuxAlly/server/global"
	bridge "github.com/JieWaZi/NeuxAlly/server/internal/knowledge"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
	"time"
)

// 知识库列表
type LibraryList struct {
	// 知识库
	Items []Library `json:"items"`
	// 总数
	Total int64 `json:"total"`
}

// 获取知识库列表
type ListLibraries struct {
	ginx.MethodGet
	// 分页参数
	// 起始位置
	Offset int `in:"query" name:"offset" validate:"min=0"`
	// 每页条数
	Limit int `in:"query" name:"limit" validate:"min=0,max=100"`
}

func (o *ListLibraries) Path() string { return "/libraries" }
func (o *ListLibraries) Output(ctx *gin.Context) (interface{}, error) {
	if err := engineAvailable(ctx); err != nil {
		return nil, err
	}
	offset, limit := pagination(Pagination{Offset: o.Offset, Limit: o.Limit})
	return ListLocalLibraries(ctx, offset, limit)
}

// ListLocalLibraries 复用本地列表响应的分页与统计组装，不额外保存目录事实。
func ListLocalLibraries(ctx context.Context, offset, limit int) (LibraryList, error) {
	rows, total, err := global.KnowledgeEngine.ListLibraries(ctx, offset, limit)
	if err != nil {
		return LibraryList{}, bridge.MapError(err)
	}
	items := []Library{}
	for _, v := range rows {
		item := libraryDTO(v)
		_, count, err := global.KnowledgeEngine.ListPages(ctx, core.PageQuery{LibraryID: v.ID, Limit: 1})
		if err != nil {
			return LibraryList{}, bridge.MapError(err)
		}
		item.PageTotal = &count
		item.PageTotalObservedAt = time.Now().UnixMilli()
		items = append(items, item)
	}
	return LibraryList{Items: items, Total: total}, nil
}
