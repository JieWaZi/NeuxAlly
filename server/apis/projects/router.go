// Package projects 提供伙伴项目 CRUD 接口。
package projects

import "github.com/shrewx/ginx"

// Router 是伙伴项目接口路由。
var Router = ginx.NewRouter(ginx.Group("/wakers/:wakerId/projects"))

func init() {
	Router.Register(&ListProjects{})
	Router.Register(&CreateProject{})
	Router.Register(&GetProject{})
	Router.Register(&UpdateProject{})
	Router.Register(&DeleteProject{})
}
