package models

import "github.com/shrewx/ginx/pkg/dbhelper"

func init() {
	dbhelper.RegisterTable(&AttentionNoticeAck{})
}

// AttentionNoticeAck 记录用户已关闭的应用内工作提醒，按稳定通知标识去重。
type AttentionNoticeAck struct {
	// NoticeID 是前端与监控共用的提醒标识，例如 approval:…、elicitation:…。
	NoticeID string `gorm:"column:notice_id;type:varchar(200);primaryKey" json:"noticeId"`
	// DismissedAt 是用户关闭提醒的 Unix 毫秒时间。
	DismissedAt int64 `gorm:"column:dismissed_at;not null" json:"dismissedAt"`
	// ViewedAt 是无原成果通知的阅读或主动清空确认时间，不能等同实际阅读。
	ViewedAt *int64 `gorm:"column:viewed_at" json:"viewedAt,omitempty"`
}

// TableName 返回应用内提醒已读表的表名。
func (AttentionNoticeAck) TableName() string {
	return "attention_notice_acks"
}
