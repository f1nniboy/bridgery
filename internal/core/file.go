package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type File struct {
	Name   string `json:"name,omitzero" luar:"name"`
	MIME   string `json:"mime,omitzero" luar:"mime"`
	URL    string `json:"url,omitzero" luar:"url"`
	Public bool   `json:"public,omitzero" luar:"public"` // url is safe to hand to other platforms
	Key    string `json:"key,omitzero" luar:"key"`
	Data   []byte `json:"-" luar:"-"` // inline bytes if no url

	Fetch func(context.Context) ([]byte, error) `json:"-" luar:"-"` // lazy bytes if no url or data, requires key
}

func (f File) Empty() bool { return f.URL == "" && len(f.Data) == 0 && f.Fetch == nil }

func (f File) Bytes(ctx context.Context) ([]byte, error) {
	if f.URL == "" {
		if len(f.Data) > 0 {
			return f.Data, nil
		}
		if f.Fetch != nil {
			return f.Fetch(ctx)
		}
		return nil, fmt.Errorf("file %q has no source", f.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", f.URL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
