// Package apis 装配 NeuxAlly Core 的 ginx HTTP 路由。
package apis

import (
	"github.com/JieWaZi/NeuxAlly/server/apis/agent"
	"github.com/JieWaZi/NeuxAlly/server/apis/ai_accounts"
	"github.com/JieWaZi/NeuxAlly/server/apis/automatic_tasks"
	"github.com/JieWaZi/NeuxAlly/server/apis/channels"
	"github.com/JieWaZi/NeuxAlly/server/apis/chats"
	eventapi "github.com/JieWaZi/NeuxAlly/server/apis/events"
	imapi "github.com/JieWaZi/NeuxAlly/server/apis/im"
	"github.com/JieWaZi/NeuxAlly/server/apis/mcp"
	memoryapi "github.com/JieWaZi/NeuxAlly/server/apis/memory"
	"github.com/JieWaZi/NeuxAlly/server/apis/middleware"
	"github.com/JieWaZi/NeuxAlly/server/apis/notifications"
	"github.com/JieWaZi/NeuxAlly/server/apis/plugins"
	"github.com/JieWaZi/NeuxAlly/server/apis/projects"
	"github.com/JieWaZi/NeuxAlly/server/apis/role_presets"
	"github.com/JieWaZi/NeuxAlly/server/apis/runtime_options"
	"github.com/JieWaZi/NeuxAlly/server/apis/skills"
	"github.com/JieWaZi/NeuxAlly/server/apis/system"
	"github.com/JieWaZi/NeuxAlly/server/apis/wakers"
	"github.com/JieWaZi/NeuxAlly/server/apis/work_board"
	"github.com/JieWaZi/NeuxAlly/server/apis/workflows"
	"github.com/shrewx/ginx"
)

// RootRouter 是版本化业务接口的根路由。
var RootRouter = ginx.NewRouter()

// desktopRouter 将桌面管理接口与执行能力的认证命名空间分开。
var desktopRouter = ginx.NewRouter(ginx.Group("/api/v1"))

func init() {
	desktopRouter.Register(&middleware.LocalTokenAuth{})
	desktopRouter.Register(system.Router)
	desktopRouter.Register(eventapi.Router)
	desktopRouter.Register(memoryapi.Router)
	desktopRouter.Register(wakers.Router)
	desktopRouter.Register(automatic_tasks.Router)
	desktopRouter.Register(automatic_tasks.GlobalRouter)
	desktopRouter.Register(projects.Router)
	desktopRouter.Register(role_presets.Router)
	desktopRouter.Register(chats.Router)
	desktopRouter.Register(channels.Router)
	desktopRouter.Register(runtime_options.Router)
	desktopRouter.Register(ai_accounts.Router)
	desktopRouter.Register(skills.Router)
	desktopRouter.Register(skills.LibraryRouter)
	desktopRouter.Register(mcp.Router)
	desktopRouter.Register(mcp.LibraryRouter)
	desktopRouter.Register(plugins.Router)
	desktopRouter.Register(plugins.WakerRouter)
	desktopRouter.Register(imapi.Router)
	desktopRouter.Register(workflows.Router)
	desktopRouter.Register(work_board.Router)
	desktopRouter.Register(notifications.Router)
	RootRouter.Register(desktopRouter)
	RootRouter.Register(agent.Router)
}
