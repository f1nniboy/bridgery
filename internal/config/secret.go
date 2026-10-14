package config

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

var envRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type Secret string // expands ${VAR} from the environment, redacted when printed

func (s *Secret) UnmarshalTOML(data []byte) error {
	var v struct {
		V string `toml:"v"`
	}
	if err := toml.Unmarshal(append([]byte("v = "), data...), &v); err != nil {
		return err
	}
	out, err := expand(v.V)
	*s = Secret(out)
	return err
}

func (Secret) String() string { return "***" }

func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"***"`), nil }

func expand(s string) (string, error) {
	var missing string
	out := envRef.ReplaceAllStringFunc(s, func(ref string) string {
		name := envRef.FindStringSubmatch(ref)[1]
		v, ok := os.LookupEnv(name)
		if !ok && missing == "" {
			missing = name
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("env var %s is not set", missing)
	}
	return out, nil
}
