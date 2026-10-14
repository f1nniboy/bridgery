package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExample(t *testing.T) {
	cfg, err := Load("../../config.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Accounts) == 0 || len(cfg.Gateways) == 0 {
		t.Errorf("example loaded empty: %+v", cfg)
	}
}

func TestLoad(t *testing.T) {
	cases := []struct {
		name, toml, err string
	}{
		{"defaults", ``, ""},
		{"accounts are opaque", "[accounts.irc.a]\nanything = 1", ""},
		{"unknown top-level field", "[runtime]\nqueue_sise = 1", "unknown field runtime.queue_sise"},
		{"unknown table", "[gateway]\nqueue_size = 1", "unknown field gateway"},
		{"unknown mode", "[[gateways]]\n[[gateways.channels]]\naccount = 'a'\nchannel = 'b'\nmode = 'both'", "unknown mode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(c.toml), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			switch {
			case c.err == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
				t.Errorf("error = %v, want %q", err, c.err)
			case err == nil && cfg.Runtime.QueueSize != 256:
				t.Errorf("defaults lost: %+v", cfg.Runtime)
			}
		})
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	var v struct {
		Size int `toml:"size"`
	}
	err := Decode(strings.NewReader("size = 1\nsiez = 2\n[extra]\nx = 1"), &v)
	if err == nil || err.Error() != "unknown field siez, extra" {
		t.Errorf("got %v", err)
	}
}
