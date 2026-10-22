package knowledge_connections

import (
	"context"
	"sort"

	"github.com/JieWaZi/NeuxAlly/server/apis/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	bridge "github.com/JieWaZi/NeuxAlly/server/internal/knowledge"
	"github.com/JieWaZi/neux-knowledge/remote"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 获取可用知识库目录
type ListAvailableLibraries struct {
	ginx.MethodGet
	// 保存位置
	Location constants.KnowledgeLibraryLocation `in:"query" name:"location"`
	// 起始位置
	Offset int `in:"query" name:"offset" validate:"min=0"`
	// 每页条数
	Limit int `in:"query" name:"limit" validate:"min=0,max=100"`
}

// catalogSource 保存单次请求观察到的来源总数，不作为持久聚合状态。
type catalogSource struct {
	// remote 为空时表示本地引擎，否则固定本次读取连接及凭据。
	remote *remoteClients
	// total 是来源按当前权限返回的真实总数。
	total int64
	// official 保存官方无分页接口本次返回的目录元数据，仅用于窗口切片。
	official []remote.Library
}

// Path 返回已认证控制面的跨来源目录入口。
func (o *ListAvailableLibraries) Path() string { return "/libraries" }

// Output 先观察真实总量，再只读取与全局分页重叠的来源窗口。
func (o *ListAvailableLibraries) Output(ctx *gin.Context) (interface{}, error) {
	requestContext := context.Context(ctx)
	if ctx.Request != nil {
		requestContext = ctx.Request.Context()
	}
	offset, limit := remotePagination(knowledge.Pagination{Offset: o.Offset, Limit: o.Limit})
	sources := []catalogSource{}
	if o.Location != constants.KnowledgeLibraryLocationRemote {
		if global.KnowledgeEngine == nil {
			return nil, status_error.KnowledgeUnsupported
		}
		_, total, err := global.KnowledgeEngine.ListLibraries(requestContext, 0, 1)
		if err != nil {
			return nil, bridge.MapError(err)
		}
		sources = append(sources, catalogSource{total: total})
	}
	if o.Location != constants.KnowledgeLibraryLocationLocal {
		if global.KnowledgeConnectionControllerManager == nil {
			return nil, status_error.KnowledgeUnsupported
		}
		connections, err := global.KnowledgeConnectionControllerManager.List(requestContext)
		if err != nil {
			return nil, err
		}
		sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
		for _, connection := range connections {
			if !connection.Enabled {
				continue
			}
			c, err := client(requestContext, connection.ID, "")
			if err != nil {
				return nil, err
			}
			source := catalogSource{remote: c}
			if c.custom != nil {
				page, err := c.custom.ListLibraries(requestContext, 0, 1)
				if err != nil {
					return nil, remoteError(err)
				}
				source.total = page.Total
			} else {
				// 官方协议仅提供无分页目录接口，沿用既有元数据读取边界。
				source.official, err = c.official.ListLibraries(requestContext)
				if err != nil {
					return nil, remoteError(err)
				}
				source.total = int64(len(source.official))
			}
			sources = append(sources, source)
		}
	}
	out := knowledge.LibraryList{Items: []knowledge.Library{}}
	remaining := int64(offset)
	for _, source := range sources {
		out.Total += source.total
		if remaining >= source.total {
			remaining -= source.total
			continue
		}
		size := min(limit-len(out.Items), int(source.total-remaining))
		if size <= 0 {
			continue
		}
		var window knowledge.LibraryList
		var err error
		if source.remote == nil {
			window, err = knowledge.ListLocalLibraries(requestContext, int(remaining), size)
		} else {
			items := []knowledge.Library{}
			if source.remote.custom != nil {
				page, e := source.remote.custom.ListLibraries(requestContext, int(remaining), size)
				if e != nil {
					return nil, remoteError(e)
				}
				for _, v := range page.Items {
					items = append(items, customLibrary(source.remote, v))
				}
			} else {
				for _, v := range source.official[int(remaining) : int(remaining)+size] {
					items = append(items, officialLibrary(source.remote, v))
				}
			}
			window, err = withLibrarySummaries(requestContext, source.remote, items, source.total)
		}
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, window.Items...)
		remaining = 0
	}
	return out, nil
}
