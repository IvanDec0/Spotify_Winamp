package soloist

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeOf(t *testing.T) {
	if got := CodeOf(nil); got != "" {
		t.Fatalf("CodeOf(nil) = %q", got)
	}
	if got := CodeOf(errors.New("plain")); got != "" {
		t.Fatalf("CodeOf(plain) = %q", got)
	}
	if got := CodeOf(ErrNotInstalled); got != CodeNotInstalled {
		t.Fatalf("CodeOf(ErrNotInstalled) = %q", got)
	}
	wrapped := fmt.Errorf("context: %w", ErrExpired)
	if got := CodeOf(wrapped); got != CodeBuildExpired {
		t.Fatalf("CodeOf(wrapped) = %q", got)
	}
	// Coded errors built during handling match the sentinel by code.
	if !errors.Is(codedError(CodeBuildExpired, "from another path", nil), ErrExpired) {
		t.Fatal("a coded build-expired error does not match ErrExpired")
	}
	if errors.Is(codedError(CodeStartFailed, "boom", nil), ErrExpired) {
		t.Fatal("different codes matched")
	}
}

func TestErrorMethods(t *testing.T) {
	base := errors.New("disk full")
	coded := codedError(CodeDownloadFailed, "cannot save the download", base)
	if coded.Error() != "cannot save the download: disk full" {
		t.Fatalf("Error() = %q", coded.Error())
	}
	if !errors.Is(coded, base) {
		t.Fatal("coded error does not unwrap to its cause")
	}
	if got := (&Error{Code: "x"}).Error(); got != "x" {
		t.Fatalf("bare error = %q", got)
	}
	var nilError *Error
	if got := nilError.Error(); got != "" {
		t.Fatalf("nil error = %q", got)
	}
	if nilError.Unwrap() != nil {
		t.Fatal("nil error unwraps to non-nil")
	}
}
