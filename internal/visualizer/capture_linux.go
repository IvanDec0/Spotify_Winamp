//go:build linux

package visualizer

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"
)

const (
	captureChunkBytes = 2048
	captureChunk      = captureChunkBytes / 2
	captureInterval   = time.Second / 30
)

var parecArgs = []string{
	"--format=s16le",
	"--rate=44100",
	"--channels=1",
	"--latency-msec=20",
	"--device=@DEFAULT_MONITOR@",
}

// Capture streams the default PulseAudio/PipeWire monitor through parec and
// emits analyzed frames at most 30 times per second. It returns when the
// context is cancelled or the capture process stops.
func Capture(ctx context.Context, emit func(Frame)) error {
	if emit == nil {
		return errors.New("visualizer: emit callback is required")
	}
	path, err := exec.LookPath("parec")
	if err != nil {
		return fmt.Errorf("visualizer: parec is not installed: %w", err)
	}

	command := exec.CommandContext(ctx, path, parecArgs...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("visualizer: open parec stdout: %w", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("visualizer: open parec stderr: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("visualizer: start parec: %w", err)
	}

	var detail stderrDetail
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			log.Printf("visualizer: parec: %s", line)
			detail.set(line)
		}
	}()

	analyzer := NewAnalyzer()
	buffer := make([]byte, captureChunkBytes)
	samples := make([]int16, captureChunk)
	var lastEmit time.Time

	for {
		_, readErr := io.ReadFull(stdout, buffer)
		if readErr != nil {
			waitErr := command.Wait()
			<-stderrDone
			if ctx.Err() != nil {
				return nil
			}
			message := detail.last()
			if waitErr != nil {
				if message != "" {
					return fmt.Errorf("visualizer: audio monitor capture failed: %w (%s)", waitErr, message)
				}
				return fmt.Errorf("visualizer: audio monitor capture failed: %w", waitErr)
			}
			return fmt.Errorf("visualizer: audio monitor capture ended: %w", readErr)
		}

		now := time.Now()
		if now.Sub(lastEmit) < captureInterval {
			continue
		}
		lastEmit = now

		for index := range samples {
			samples[index] = int16(binary.LittleEndian.Uint16(buffer[index*2:]))
		}
		emit(analyzer.Analyze(samples))
	}
}

// stderrDetail keeps the most recent diagnostically useful parec line. The
// capture loop only reads it after the stderr goroutine has finished, so no
// lock is necessary.
type stderrDetail struct {
	line string
}

func (d *stderrDetail) set(line string) {
	if len(line) > 240 {
		line = line[:240]
	}
	d.line = line
}

func (d *stderrDetail) last() string {
	return d.line
}
