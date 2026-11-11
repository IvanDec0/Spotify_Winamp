package developer

import "errors"

// Stable error codes carried by Error. They are identifiers, not messages:
// callers and the frontend may localize them.
const (
	CodeLoginRequired   = "developer_login_required"
	CodePremiumRequired = "developer_premium_required"
	CodeAppLimit        = "developer_app_limit"
	CodeFailed          = "developer_failed"
)

// Error is a developer automation error carrying a stable code.
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

// Is matches coded errors by code.
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

// CodeOf returns the stable code carried by err, or "" when err is nil or has
// no code.
func CodeOf(err error) string {
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// Sentinel errors used by the public API.
var (
	ErrLoginRequired = &Error{
		Code:    CodeLoginRequired,
		Message: "sign in to the Spotify Developer dashboard first",
	}
	ErrPremiumRequired = &Error{
		Code:    CodePremiumRequired,
		Message: "Spotify Premium is required for Soloist",
	}
	ErrAppLimit = &Error{
		Code:    CodeAppLimit,
		Message: "the Spotify account reached its developer app limit",
	}
)

func codedError(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}
