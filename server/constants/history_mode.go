package constants

//go:generate toolx gen enum HistoryMode

// HistoryMode 是新会话取得此前聊天上下文的方式。
type HistoryMode string

const (
	// HistoryModeDefault 沿用近期完整对话作为默认策略。
	HistoryModeDefault HistoryMode = ""
	// HistoryModeRecent 使用有限条近期完整对话。
	HistoryModeRecent HistoryMode = "recent"
	// HistoryModeSummary 使用旧会话生成的隐藏交接内容。
	HistoryModeSummary HistoryMode = "summary"
)
