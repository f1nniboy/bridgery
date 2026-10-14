package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/sync/singleflight"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
)

type store interface {
	Get(ctx context.Context, key, name string) (string, bool)
	Put(ctx context.Context, key, name, mime string, data []byte) (string, error)
}

type backend interface {
	store
	config.Backend
}

var Registry = map[string]func() backend{
	"local": func() backend { return &local{} },
	"s3":    func() backend { return &s3{} },
}

func Open(cfg config.Backends) (core.Media, error) {
	if len(cfg) == 0 {
		return &resolver{}, nil
	}
	b, err := config.OpenBackend(cfg, Registry, "")
	if err != nil {
		return nil, err
	}
	return &resolver{backend: b}, nil
}

type resolver struct {
	backend store
	group   singleflight.Group
}

func (r *resolver) URL(ctx context.Context, f core.File) (string, error) {
	if f.URL != "" && f.Public {
		return f.URL, nil
	}
	if f.Empty() || r.backend == nil {
		return "", nil
	}

	var flight, key string
	switch {
	case f.Key != "":
		key, flight = hashHex([]byte(f.Key)), "k:"+f.Key
	case f.URL != "":
		flight = "u:" + f.URL
	default:
		key = hashHex(f.Data)
		flight = "b:" + key
	}

	v, err, _ := r.group.Do(flight, func() (any, error) {
		var data []byte
		if key == "" {
			b, err := f.Bytes(ctx)
			if err != nil {
				return "", err
			}
			key, data = hashHex(b), b
		}

		if url, ok := r.backend.Get(ctx, key, f.Name); ok {
			return url, nil
		}

		if data == nil {
			b, err := f.Bytes(ctx)
			if err != nil {
				return "", err
			}
			data = b
		}
		return r.backend.Put(ctx, key, f.Name, f.MIME, data)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
