package engine

// Stable engine state codes exposed through AudioState.ErrorCode. The frontend
// translates them; the raw Error string stays the diagnostic detail.
const (
	ErrorCodeNotInstalled = "engine_not_installed"
	ErrorCodeConfig       = "engine_config_error"
	ErrorCodeStartFailed  = "engine_start_failed"
	ErrorCodeNotReady     = "engine_not_ready"
	ErrorCodeStopped      = "engine_stopped"
	ErrorCodeDisabled     = "engine_disabled"
	ErrorCodeNotRunning   = "engine_not_running"
)

// CodedError pairs a stable code with a diagnostic message for operations that
// need the engine but cannot reach it.
type CodedError struct {
	Code    string
	Message string
}

func (e *CodedError) Error() string { return e.Message }

// NewCodedError builds an engine error with a stable code.
func NewCodedError(code, message string) error {
	return &CodedError{Code: code, Message: message}
}
