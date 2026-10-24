// Package websession 提供本机 Web 的一次性配对和会话退出入口。
package websession

import "github.com/shrewx/ginx"

// Router 与 Bearer 管理 API 共用 Core，只有配对入口允许匿名访问。
var Router = ginx.NewRouter(ginx.Group("/api/web"))

func init() {
	Router.Register(&PairSession{})
	Router.Register(&GetSession{})
	Router.Register(&DeleteSession{})
}
