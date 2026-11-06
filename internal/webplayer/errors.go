package webplayer

import "errors"

// Stable error codes carried by Error. They are stable identifiers, not
// messages: callers and the frontend may localize them.
const (
	CodeInvalidManifest  = "webplayer_invalid_manifest"
	CodeUnsupported      = "webplayer_unsupported_platform"
	CodeDownloadFailed   = "webplayer_download_failed"
	CodeChecksumMismatch = "webplayer_checksum_mismatch"
	CodeInstallFailed    = "webplayer_install_failed"
	CodeNotInstalled     = "webplayer_not_installed"
	CodeStartFailed      = "webplayer_start_failed"
	CodeInvalidArgument  = "webplayer_invalid_argument"
	CodeProtocol         = "webplayer_protocol_error"
)

// Error is a web player error carrying a stable code.
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

// CodeOf returns the stable code carried by err, or "" when err is nil or has
// no code.
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
	ErrInvalidManifest     = &Error{Code: CodeInvalidManifest, Message: "the web player manifest is invalid"}
	ErrUnsupportedPlatform = &Error{Code: CodeUnsupported, Message: "the web player runtime is not available for this platform"}
	ErrChecksumMismatch    = &Error{Code: CodeChecksumMismatch, Message: "the downloaded web player runtime failed SHA-256 verification"}
	ErrNotInstalled        = &Error{Code: CodeNotInstalled, Message: "the web player runtime is not installed"}
	ErrNotConnected        = &Error{Code: CodeProtocol, Message: "the web player control socket is not connected"}
	ErrInvalidArgument     = &Error{Code: CodeInvalidArgument, Message: "the web player argument is invalid"}
)

// codedError builds an Error with a code, a message and an optional cause.
func codedError(code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}
