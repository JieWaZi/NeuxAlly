// Package mcp 提供伙伴 MCP 管理接口。
package mcp

import "github.com/shrewx/ginx"

// Router 是伙伴 MCP 接口路由。
var Router = ginx.NewRouter(ginx.Group("/wakers"))

func init() {
	Router.Register(&ListWakerConnectors{})
	Router.Register(&CreateWakerConnector{})
	Router.Register(&GetWakerConnector{})
	Router.Register(&UpdateWakerConnector{})
	Router.Register(&DeleteWakerConnector{})
	Router.Register(&SetWakerConnectorEnabled{})
	Router.Register(&TestWakerConnector{})
}
