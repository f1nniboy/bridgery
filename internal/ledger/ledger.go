package ledger

import (
	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
)

type backend interface {
	core.Ledger
	config.Backend
}

var Registry = map[string]func() backend{
	"memory": func() backend { return &memory{MaxSize: defaultMaxSize} },
}

func Open(cfg config.Backends) (core.Ledger, error) {
	return config.OpenBackend(cfg, Registry, "memory")
}
