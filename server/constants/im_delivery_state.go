package constants

//go:generate toolx gen enum IMDeliveryState

// IMDeliveryState 定义 IM 集成的稳定业务值。
type IMDeliveryState string

const (
	// IMDeliveryStateUnspecified 表示尚无投递状态。
	IMDeliveryStateUnspecified IMDeliveryState = ""
	// IMDeliveryStatePending 表示等待平台发送。
	IMDeliveryStatePending IMDeliveryState = "pending"
	// IMDeliveryStateSending 表示已开始外部发送。
	IMDeliveryStateSending IMDeliveryState = "sending"
	// IMDeliveryStateSent 表示平台已确认发送。
	IMDeliveryStateSent IMDeliveryState = "sent"
	// IMDeliveryStateFailed 表示平台明确拒绝发送。
	IMDeliveryStateFailed IMDeliveryState = "failed"
	// IMDeliveryStateUnknown 表示外部发送结果不确定需要核对。
	IMDeliveryStateUnknown IMDeliveryState = "unknown"
)
