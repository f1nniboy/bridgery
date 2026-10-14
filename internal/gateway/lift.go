package gateway

import (
	"slices"

	"github.com/f1nniboy/bridgery/internal/core"
)

func (g *Gateway) lift(in core.Inbound) (core.Event, bool) {
	ev := core.Event{Kind: in.Kind, Origin: in.Origin, Platform: g.accounts[in.Origin.Account].Platform, Message: in.Message}

	switch in.Kind {
	case core.KindPost:
		if in.NativeID != "" {
			if _, echo := g.ledger.Canonical(in.Origin, in.NativeID); echo {
				return core.Event{}, false // already seen, bridges drop our own echoes before this
			}
		}
		ev.ID = core.NewID()
		if in.NativeID != "" {
			g.ledger.Bind(ev.ID, in.Origin, []string{in.NativeID})
		}

	case core.KindEdit:
		id, ok := g.ledger.Canonical(in.Origin, in.NativeID)
		if !ok {
			return core.Event{}, false
		}
		ev.ID = id
		if rec, found := g.ledger.Get(id); found {
			if slices.Equal(rec.Content, in.Content) {
				return core.Event{}, false // not a real edit
			}
		}

	case core.KindDelete:
		id, ok := g.ledger.Canonical(in.Origin, in.NativeID)
		if !ok {
			return core.Event{}, false
		}
		rec, found := g.ledger.Get(id)
		if !found {
			return core.Event{}, false // already applied, e.g. the echo of a delete we relayed
		}
		ev.ID = id
		ev.Author = rec.Author
		ev.Content, ev.Files = rec.Content, rec.Files // carry what was deleted for hooks
		g.ledger.Delete(id)
	}

	if in.ParentID != "" {
		if id, ok := g.ledger.Canonical(in.Origin, in.ParentID); ok {
			ev.ParentID = id
		}
	}

	return ev, true
}
