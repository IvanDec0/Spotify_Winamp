//go:build linux

package visualizer

import (
	"context"
	"strings"
	"testing"
)

func TestCaptureReportsMissingParec(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := Capture(context.Background(), func(Frame) {})
	if err == nil {
		t.Fatal("expected an error when parec is missing")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("error = %v, want a clear missing-parec message", err)
	}
}

func TestCaptureRejectsNilCallback(t *testing.T) {
	err := Capture(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error for a nil emit callback")
	}
}
