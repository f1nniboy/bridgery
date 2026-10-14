package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type local struct {
	Dir     string `toml:"dir,commented" comment:"directory to write re-hosted files into"`
	BaseURL string `toml:"base_url,commented" comment:"public base URL serving that directory\nyou serve this directory yourself with a web server"`
}

func (l *local) Open() error {
	if l.Dir == "" || l.BaseURL == "" {
		return errors.New("backend requires dir and base_url")
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	l.BaseURL = strings.TrimRight(l.BaseURL, "/")
	return nil
}

func (l *local) Example() any {
	return &local{Dir: "/var/lib/bridgery/media", BaseURL: "https://domain.tld/media"}
}

func (l *local) Get(_ context.Context, key, name string) (string, bool) {
	path, url := l.locate(key, name)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return url, true
}

func (l *local) Put(_ context.Context, key, name, _ string, data []byte) (string, error) {
	path, url := l.locate(key, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return url, nil
}

func (l *local) locate(key, name string) (path, url string) {
	name = safeName(name)
	return filepath.Join(l.Dir, key, name), l.BaseURL + "/" + key + "/" + name
}

func safeName(name string) string {
	name = unsafeName.ReplaceAllString(filepath.Base(name), "_")
	if strings.Trim(name, "._-") == "" || !filepath.IsLocal(name) {
		return "file"
	}
	return name
}
