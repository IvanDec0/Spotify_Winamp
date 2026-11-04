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
		deviceName:   "Cadence",
		audioBackend: "pulseaudio",
		audioDevice:  "default",
		bitrate:      320,
	}

	if err := runtime.writeConfig(12345, dir); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "prefer_firewall_friendly_ports: true") {
		t.Fatalf("config does not prefer firewall-friendly ports:\n%s", data)
	}
}
