package constants

//go:generate toolx gen enum AutomaticScheduleKind

// AutomaticScheduleKind 是分钟精度的定时规则种类。
type AutomaticScheduleKind string

const (
	// AutomaticScheduleKindUnspecified 表示未配置定时规则。
	AutomaticScheduleKindUnspecified AutomaticScheduleKind = ""
	// AutomaticScheduleKindOnce 表示指定当地日期时间的一次性计划。
	AutomaticScheduleKindOnce AutomaticScheduleKind = "once"
	// AutomaticScheduleKindHourly 表示每小时的指定分钟。
	AutomaticScheduleKindHourly AutomaticScheduleKind = "hourly"
	// AutomaticScheduleKindDaily 表示每天的指定时间。
	AutomaticScheduleKindDaily AutomaticScheduleKind = "daily"
	// AutomaticScheduleKindWeekly 表示每周指定星期的时间。
	AutomaticScheduleKindWeekly AutomaticScheduleKind = "weekly"
	// AutomaticScheduleKindMonthly 表示每月指定日期的时间。
	AutomaticScheduleKindMonthly AutomaticScheduleKind = "monthly"
	// AutomaticScheduleKindCron 表示自定义五字段 Cron。
	AutomaticScheduleKindCron AutomaticScheduleKind = "cron"
)
