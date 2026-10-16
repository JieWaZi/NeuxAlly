package constants

//go:generate toolx gen enum ChannelMessageActor

// ChannelMessageActor 是 Team 持久输入和聚合时间线使用的封闭作者类型。
type ChannelMessageActor string

const (
	// ChannelMessageActorUser 表示团队输入来自真实用户。
	ChannelMessageActorUser ChannelMessageActor = "user"
	// ChannelMessageActorLead 表示成员 Chat 输入来自团队负责人内部派发。
	ChannelMessageActorLead ChannelMessageActor = "team_lead"
	// ChannelMessageActorMemberReport 表示负责人 Chat 中由成员提交的结构化结果报告。
	ChannelMessageActorMemberReport ChannelMessageActor = "team_member"
	// ChannelMessageActorSystem 表示负责人 Chat 输入来自团队结果通知。
	ChannelMessageActorSystem ChannelMessageActor = "team_system"
)
