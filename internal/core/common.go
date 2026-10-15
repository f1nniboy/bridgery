package core

import (
	"context"
	"log/slog"
)

type Common struct {
	inboundCh chan Inbound
	statusCh  chan Status

	Log   *slog.Logger
	Media Media
}

func NewCommon(log *slog.Logger, media Media) *Common {
	return &Common{inboundCh: make(chan Inbound, 64), statusCh: make(chan Status, 16), Log: log, Media: media}
}

func (c *Common) Send(in Inbound) {
	select {
	case c.inboundCh <- in:
	default:
		c.Log.Warn("inbound chan full, dropping", "kind", in.Kind)
	}
}

func (c *Common) Inbound() <-chan Inbound {
	return c.inboundCh
}

func (c *Common) Report(ctx context.Context, m Status) {
	if ctx.Err() != nil {
		return
	}
	select {
	case c.statusCh <- m:
	case <-ctx.Done():
	}
}

func (c *Common) Status() <-chan Status {
	return c.statusCh
}
