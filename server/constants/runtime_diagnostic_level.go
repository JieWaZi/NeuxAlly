package constants

//go:generate toolx gen enum RuntimeDiagnosticLevel

// RuntimeDiagnosticLevel 表示 Runtime 诊断对调用方的影响级别。
type RuntimeDiagnosticLevel string

const (
	// RuntimeDiagnosticLevelUnspecified 表示未指定诊断影响级别。
	RuntimeDiagnosticLevelUnspecified RuntimeDiagnosticLevel = ""
	// RuntimeDiagnosticLevelInfo 表示不影响调用的补充信息。
	RuntimeDiagnosticLevelInfo RuntimeDiagnosticLevel = "info"
	// RuntimeDiagnosticLevelWarning 表示已降级但仍可能继续调用。
	RuntimeDiagnosticLevelWarning RuntimeDiagnosticLevel = "warning"
	// RuntimeDiagnosticLevelError 表示当前 Runtime 不能安全启动。
	RuntimeDiagnosticLevelError RuntimeDiagnosticLevel = "error"
)
