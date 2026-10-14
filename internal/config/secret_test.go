package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretExpand(t *testing.T) {
	t.Setenv("BRIDGERY_TEST_TOKEN", "abc")
	t.Setenv("BRIDGERY_TEST_EMPTY", "")
	cases := []struct {
		in, want, err string
	}{
		{"plain", "plain", ""},
		{"${BRIDGERY_TEST_TOKEN}", "abc", ""},
		{"Bot ${BRIDGERY_TEST_TOKEN}!", "Bot abc!", ""},
		{"${BRIDGERY_TEST_EMPTY}", "", ""},
		{"pa$$word $BRIDGERY_TEST_TOKEN", "pa$$word $BRIDGERY_TEST_TOKEN", ""},
		{"${BRIDGERY_TEST_UNSET}", "", "BRIDGERY_TEST_UNSET is not set"},
	}
	for _, c := range cases {
		got, err := expand(c.in)
		switch {
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%q: error = %v, want %q", c.in, err, c.err)
		case c.err == "" && (err != nil || got != c.want):
			t.Errorf("%q: got %q, %v, want %q", c.in, got, err, c.want)
		}
	}
}

func TestSecretRedacted(t *testing.T) {
	s := Secret("hunter2")
	if got := fmt.Sprint(s); got != "***" {
		t.Errorf("fmt: %q", got)
	}
	for _, h := range []func(*bytes.Buffer) slog.Handler{
		func(b *bytes.Buffer) slog.Handler { return slog.NewTextHandler(b, nil) },
		func(b *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(b, nil) },
	} {
		var buf bytes.Buffer
		slog.New(h(&buf)).Info("x", "secret", s, "cfg", struct{ Token Secret }{s})
		if strings.Contains(buf.String(), "hunter2") {
			t.Errorf("leaked: %s", buf.String())
		}
	}
}

func TestSecretInConfig(t *testing.T) {
	t.Setenv("BRIDGERY_TEST_KEY", "k")
	var v struct {
		Key Secret `toml:"key"`
	}
	if err := Decode(strings.NewReader(`key = '${BRIDGERY_TEST_KEY}'`), &v); err != nil || v.Key != "k" {
		t.Errorf("got %q, %v", string(v.Key), err)
	}
	err := Decode(strings.NewReader(`key = '${BRIDGERY_TEST_NOPE}'`), &v)
	if err == nil || !strings.Contains(err.Error(), "BRIDGERY_TEST_NOPE is not set") {
		t.Errorf("got %v", err)
	}
	if err := Decode(strings.NewReader(`key = """multi ${BRIDGERY_TEST_KEY}"""`), &v); err != nil || v.Key != "multi k" {
		t.Errorf("multiline: got %q, %v", string(v.Key), err)
	}
}
