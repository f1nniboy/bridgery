package config

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2/unstable"
)

type Backends map[string]unstable.RawMessage // backend name -> its settings

type Backend interface {
	Open() error
	Example() any
}

func OpenBackend[T Backend](cfg Backends, registry map[string]func() T, def string) (T, error) {
	var zero T
	name, raw := def, unstable.RawMessage(nil)
	switch len(cfg) {
	case 0:
	case 1:
		for n, r := range cfg {
			name, raw = n, r
		}
	default:
		return zero, errors.New("only one backend may be configured")
	}

	newBackend, ok := registry[name]
	if !ok {
		return zero, fmt.Errorf("unknown backend %q", name)
	}
	b := newBackend()
	if len(raw) > 0 {
		if err := Decode(bytes.NewReader(raw), b); err != nil {
			return zero, fmt.Errorf("%s: %w", name, err)
		}
	}
	if err := b.Open(); err != nil {
		return zero, fmt.Errorf("%s: %w", name, err)
	}
	return b, nil
}

func ExampleBackend[T Backend](registry map[string]func() T, name string) any {
	if newBackend, ok := registry[name]; ok {
		return newBackend().Example()
	}
	return nil
}
