package config

import (
	"errors"
	"testing"

	"github.com/pelletier/go-toml/v2/unstable"
)

type fake struct {
	Size   int `toml:"size"`
	opened bool
	fail   bool
}

func (f *fake) Open() error {
	if f.fail {
		return errors.New("boom")
	}
	f.opened = true
	return nil
}

func (f *fake) Example() any { return &fake{Size: 1} }

var registry = map[string]func() *fake{
	"a":   func() *fake { return &fake{Size: 7} },
	"bad": func() *fake { return &fake{fail: true} },
}

func TestOpenBackend(t *testing.T) {
	t.Run("empty uses the default with its defaults", func(t *testing.T) {
		b, err := OpenBackend(nil, registry, "a")
		if err != nil || !b.opened || b.Size != 7 {
			t.Errorf("got %+v, %v", b, err)
		}
	})

	t.Run("settings decode over defaults", func(t *testing.T) {
		b, err := OpenBackend(Backends{"a": unstable.RawMessage("size = 3")}, registry, "")
		if err != nil || b.Size != 3 {
			t.Errorf("got %+v, %v", b, err)
		}
	})

	for name, cfg := range map[string]Backends{
		"unknown backend": {"nope": nil},
		"multiple":        {"a": nil, "bad": nil},
		"open error":      {"bad": nil},
		"bad settings":    {"a": unstable.RawMessage(`size = "x"`)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := OpenBackend(cfg, registry, ""); err == nil {
				t.Error("expected error")
			}
		})
	}
}
