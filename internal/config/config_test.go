package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		{"zero queue size", "[runtime]\nqueue_size = 0", "queue_size must be greater than 0"},
		{"zero timeout", "[runtime]\ntimeout = '0s'", "timeout must be greater than 0"},
		{"unknown mode", "[[gateways]]\n[[gateways.channels]]\naccount = 'a'\nchannel = 'b'\nmode = 'inout'", "unknown mode"},
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

func TestDurationText(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{time.Minute, "1m"},
		{24 * time.Hour, "24h"},
		{90 * time.Minute, "1h30m"},
		{time.Hour + time.Second, "1h0m1s"},
	}
	for _, c := range cases {
		b, _ := Duration(c.in).MarshalText()
		var back Duration
		if err := back.UnmarshalText(b); err != nil || string(b) != c.want || time.Duration(back) != c.in {
			t.Errorf("%v: text %q (want %q), parsed back %v, %v", c.in, b, c.want, time.Duration(back), err)
		}
	}
}
