package constants

//go:generate toolx gen enum AutomaticRunAdmission

// AutomaticRunAdmission 记录触发机会进入 Chat 权威事实的接纳阶段。
type AutomaticRunAdmission string

const (
	// AutomaticRunAdmissionUnspecified 表示尚无接纳阶段。
	AutomaticRunAdmissionUnspecified AutomaticRunAdmission = ""
	// AutomaticRunAdmissionPending 表示已领取机会但尚未确认 Chat。
	AutomaticRunAdmissionPending AutomaticRunAdmission = "pending"
	// AutomaticRunAdmissionAccepted 表示 Chat 与 Execution 已持久建立。
	AutomaticRunAdmissionAccepted AutomaticRunAdmission = "accepted"
	// AutomaticRunAdmissionFailed 表示接纳前的配置或执行准备失败。
	AutomaticRunAdmissionFailed AutomaticRunAdmission = "failed"
	// AutomaticRunAdmissionInterrupted 表示接纳过程中重启且未建立 Chat。
	AutomaticRunAdmissionInterrupted AutomaticRunAdmission = "interrupted"
	// AutomaticRunAdmissionSkipped 表示该计划机会因忙碌或错过被跳过。
	AutomaticRunAdmissionSkipped AutomaticRunAdmission = "skipped"
)
