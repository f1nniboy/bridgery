package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var fileClient = &http.Client{Timeout: 5 * time.Second}

type File struct {
	Name   string `luar:"name"`
	MIME   string `luar:"mime"`
	URL    string `luar:"url"`
	Public bool   `luar:"public"` // url is safe to hand to other platforms
	Key    string `luar:"key"`
	Data   []byte `luar:"-"` // inline bytes if no url
}

func (f File) Empty() bool { return f.URL == "" && len(f.Data) == 0 }

func (f File) Bytes(ctx context.Context) ([]byte, error) {
	if f.URL == "" {
		if len(f.Data) > 0 {
			return f.Data, nil
		}
		return nil, fmt.Errorf("file %q has no source", f.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fileClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", f.URL, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
