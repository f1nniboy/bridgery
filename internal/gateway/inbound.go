package gateway

import (
	"context"
	"slices"

	"github.com/f1nniboy/bridgery/internal/core"
)

func (r *Router) ingest(ctx context.Context, account string, inbound <-chan core.Inbound) {
	for {
		select {
		case <-ctx.Done():
			// drain what's already buffered so shutdown doesn't discard it
			for {
				select {
				case in := <-inbound:
					r.handle(in, account)
				default:
					return
				}
			}
		case in := <-inbound:
			r.handle(in, account)
		}
	}
}

func (r *Router) handle(in core.Inbound, account string) {
	in.Origin.Account = account
	ev, ok := r.lift(in)
	if !ok {
		return
	}
	if r.hooks != nil {
		if ev, ok = r.hooks.Receive(ev); !ok {
			return // cancelled by hook
		}
	}
	if ev.Kind == core.KindEdit {
		if rec, found := r.ledger.Get(ev.ID); found && slices.Equal(rec.Content, ev.Content) {
			return // edit with same content as before, so skip
		}
	}
	if ev.Kind == core.KindPost || ev.Kind == core.KindEdit {
		r.ledger.Put(ev.ID, stripData(ev.Message))
	}
	for _, dst := range r.routes[ev.Origin] {
		select {
		case r.queues[dst] <- ev:
		default:
			r.log.Warn("queue full, dropping event", "dst", dst, "kind", ev.Kind)
		}
	}
}

func (r *Router) lift(in core.Inbound) (core.Event, bool) {
	ev := core.Event{Kind: in.Kind, Origin: in.Origin, Platform: r.accounts[in.Origin.Account].Platform, Message: in.Message}

	switch in.Kind {
	case core.KindPost:
		if in.NativeID != "" {
			if _, echo := r.ledger.Canonical(in.Origin, in.NativeID); echo {
				return core.Event{}, false // already seen, bridges drop our own echoes before this
			}
		}
		ev.ID = core.NewID()
		if in.NativeID != "" {
			r.ledger.Bind(ev.ID, in.Origin, []string{in.NativeID})
		}

	case core.KindEdit:
		id, ok := r.ledger.Canonical(in.Origin, in.NativeID)
		if !ok {
			return core.Event{}, false
		}
		ev.ID = id

	case core.KindDelete:
		id, ok := r.ledger.Canonical(in.Origin, in.NativeID)
		if !ok {
			return core.Event{}, false
		}
		rec, found := r.ledger.Get(id)
		if !found {
			return core.Event{}, false // already applied, e.g. the echo of a delete we relayed
		}
		ev.ID = id
		ev.Author = rec.Author
		ev.Content, ev.Files = rec.Content, rec.Files // carry what was deleted for hooks
		r.ledger.Delete(id)
	}

	if in.ParentID != "" {
		if id, ok := r.ledger.Canonical(in.Origin, in.ParentID); ok {
			ev.ParentID = id
		}
	}

	return ev, true
}

func stripData(m core.Message) core.Message {
	m.Author.Avatar.Data = nil
	if len(m.Files) > 0 {
		files := slices.Clone(m.Files)
		for i := range files {
			files[i].Data = nil
		}
		m.Files = files
	}
	return m
}
