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
