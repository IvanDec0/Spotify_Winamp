package soloist

import "errors"

// Stable error codes exposed through State.ErrorCode and CodeOf. The frontend
// localizes these codes; the messages are English fallbacks.
const (
	CodeNotInstalled      = "soloist_not_installed"
	CodeUnsupported       = "soloist_unsupported_platform"
	CodeAPIKeyMissing     = "soloist_api_key_missing"
	CodeAPIKeyInvalid     = "soloist_api_key_invalid"
	CodeAPIKeyRejected    = "soloist_api_key_rejected"
	CodeNotLoggedIn       = "soloist_not_logged_in"
	CodePairingRequired   = "soloist_pairing_required"
	CodeBuildExpired      = "soloist_build_expired"
	CodeStartFailed       = "soloist_start_failed"
	CodeDownloadFailed    = "soloist_download_failed"
	CodeProtocol          = "soloist_protocol_error"
	CodeUnsupportedAction = "soloist_unsupported"
)

// Error is a Soloist error carrying a stable code.
type Error struct {
	Code    string
	Message string
	Err     error
}

// Error implements error.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Message != "" && e.Err != nil:
		return e.Message + ": " + e.Err.Error()
	case e.Message != "":
		return e.Message
	case e.Err != nil:
		return e.Err.Error()
	default:
		return e.Code
	}
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is matches coded errors by code, so errors.Is works with both the sentinel
// values and coded errors built while handling a failure.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	other, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code != "" && e.Code == other.Code
}

// CodeOf returns the stable Soloist error code carried by err, or "" when err
// is nil or has no code.
func CodeOf(err error) string {
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// Sentinel errors used by the public API. Use errors.Is to test them; CodeOf
// also works because they are coded errors.
var (
	ErrNotInstalled        = &Error{Code: CodeNotInstalled, Message: "Spotify Soloist is not installed"}
	ErrExpired             = &Error{Code: CodeBuildExpired, Message: "this Spotify Soloist build has expired"}
	ErrAPIKeyMissing       = &Error{Code: CodeAPIKeyMissing, Message: "the Spotify Soloist API key is missing"}
	ErrNotLoggedIn         = &Error{Code: CodeNotLoggedIn, Message: "Spotify Soloist is not logged in"}
	ErrUnsupportedPlatform = &Error{Code: CodeUnsupported, Message: "Spotify Soloist is not available for this platform"}
	ErrPairingRequired     = &Error{Code: CodePairingRequired, Message: "Spotify Soloist needs to be paired with a Spotify account"}
)

// codedError builds an Error with a code and message.
func codedError(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}
