//go:build !linux

package visualizer

import (
	"context"
	"errors"
)

// Capture is unavailable outside Linux: the monitor source depends on
// PulseAudio/PipeWire and parec.
func Capture(_ context.Context, _ func(Frame)) error {
	return errors.New("visualizer: audio capture is not supported on this platform")
}
