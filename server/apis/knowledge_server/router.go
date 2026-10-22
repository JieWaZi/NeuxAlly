// Package knowledge_server 只向本机管理控制面开放知识服务策略重载。
package knowledge_server

import "github.com/shrewx/ginx"

// Router 继承本机管理令牌，不接受远端知识凭据。
var Router = ginx.NewRouter(ginx.Group("/knowledge/server"))

func init() { Router.Register(&ReloadPolicy{}) }
