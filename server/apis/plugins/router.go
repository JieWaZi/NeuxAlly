package plugins

import "github.com/shrewx/ginx"

// Router 提供插件库与共享资源管理入口。
var Router = ginx.NewRouter(ginx.Group("/plugins"))

// WakerRouter 提供伙伴真实插件安装入口。
var WakerRouter = ginx.NewRouter(ginx.Group("/wakers"))

func init() {
	Router.Register(&ListPluginCatalog{})
	Router.Register(&PrepareCatalogPlugin{})
	Router.Register(&ListPlugins{})
	Router.Register(&GetPlugin{})
	Router.Register(&ScanPlugin{})
	Router.Register(&PreparePlugin{})
	Router.Register(&InstallPlugin{})
	Router.Register(&UpdatePluginConfiguration{})
	Router.Register(&UpdatePlugin{})
	Router.Register(&DeletePlugin{})
	Router.Register(&GetPluginAuthorization{})
	Router.Register(&ConfigurePluginAuthorization{})
	Router.Register(&StartPluginAuthorization{})
	Router.Register(&CancelPluginAuthorization{})
	Router.Register(&LogoutPluginAuthorization{})
	WakerRouter.Register(&ListWakerPlugins{})
	WakerRouter.Register(&SetWakerPluginEnabled{})
	WakerRouter.Register(&UninstallWakerPlugin{})
}
