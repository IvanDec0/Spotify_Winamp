package golibrespot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigPrefersFirewallFriendlyPorts(t *testing.T) {
	dir := t.TempDir()
	runtime := &Runtime{
		deviceName:    "Cadence",
		audioBackend:  "pulseaudio",
		audioDevice:   "default",
		bitrate:       320,
		deviceIsGroup: true,
	}

	if err := runtime.writeConfig(12345, dir); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, want := range []string{
		"device_is_group: true",
		"image_size: default",
		"prefer_firewall_friendly_ports: true",
		"accesspoint_ports: [80, 443]",
		"metadata:",
		"max_tracks: 800",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("config does not contain %q:\n%s", want, data)
		}
	}
}

func TestNewRuntimeValidatesImageSize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "default"},
		{name: "default", input: "default", want: "default"},
		{name: "small", input: "small", want: "small"},
		{name: "large", input: "large", want: "large"},
		{name: "xlarge", input: "xlarge", want: "xlarge"},
		{name: "unknown", input: "huge", want: "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := NewRuntime(RuntimeOptions{ImageSize: test.input})
			if runtime.imageSize != test.want {
				t.Fatalf("imageSize = %q, want %q", runtime.imageSize, test.want)
			}
		})
	}
}

func TestRuntimeParsesEngineVersionFromLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "go-librespot.log")
	line := `time="2026-11-04T08:55:32Z" level=info msg="running go-librespot 0.10.6-0.20261104085532-16b65bb4cd2b"` + "\n"
	if err := os.WriteFile(logPath, []byte(line), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	runtime := &Runtime{logPath: logPath}
	const want = "0.10.6-0.20261104085532-16b65bb4cd2b"
	if got := runtime.Version(); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
	if got := runtime.Version(); got != want {
		t.Fatalf("cached Version() = %q, want %q", got, want)
	}
}

func TestRuntimeVersionEmptyWhenLogMissing(t *testing.T) {
	runtime := &Runtime{logPath: filepath.Join(t.TempDir(), "missing.log")}
	if got := runtime.Version(); got != "" {
		t.Fatalf("Version() = %q, want empty", got)
	}
}

func TestRuntimeVersionDoesNotPanic(t *testing.T) {
	var runtime Runtime
	if got := runtime.Version(); got != "" {
		t.Fatalf("Version() = %q, want empty", got)
	}
}
