// Package im 提供受本机令牌保护的连接、聊天授权和交付管理接口。
package im

import "github.com/shrewx/ginx"

// Router 是 IM 管理路由。
var Router = ginx.NewRouter(ginx.Group("/im"))

func init() {
	Router.Register(&ListIMConnections{})
	Router.Register(&CreateIMConnection{})
	Router.Register(&UpdateIMConnection{})
	Router.Register(&DeleteIMConnection{})
	Router.Register(&GetIMConnectionDependencies{})
	Router.Register(&ListIMBindings{})
	Router.Register(&UpdateIMBinding{})
	Router.Register(&ListIMConversations{})
	Router.Register(&SelectIMConversation{})
	Router.Register(&StartIMLogin{})
	Router.Register(&GetIMLogin{})
	Router.Register(&CancelIMLogin{})
	Router.Register(&VerifyIMLogin{})
	Router.Register(&GetIMStatus{})
}
