package notifications

import "github.com/shrewx/ginx"

// Router 注册统一通知查询与确认操作，不执行业务审批。
var Router = ginx.NewRouter(ginx.Group("/notifications"))

func init() {
	Router.Register(&GetNotificationSnapshot{})
	Router.Register(&ListNotifications{})
	Router.Register(&GetNotification{})
	Router.Register(&DismissNotification{})
	Router.Register(&ViewNotification{})
	Router.Register(&ClearNotificationUnread{})
}
