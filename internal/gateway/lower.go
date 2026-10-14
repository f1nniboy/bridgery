package gateway

import (
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
)

func (g *Gateway) lower(ev core.Event, dst core.Address, caps core.CapSet) (core.Delivery, bool) {
	origKind := ev.Kind

	var parent *core.Message
	var parentIDs []string
	var parentOK bool
	if ev.ParentID != "" {
		if rec, ok := g.ledger.Get(ev.ParentID); ok {
			parent = &rec
		}
		parentIDs, parentOK = g.ledger.Native(ev.ParentID, dst)
	}

	if g.hooks != nil {
		t := hook.Target{Platform: g.accounts[dst.Account].Platform, Address: dst, Caps: caps, Parent: parent}
		if ev.ParentID != "" && !parentOK {
			t.Caps = caps.Without(core.CapReply) // has parent id but can't resolve
		}
		var keep bool
		if ev, keep = g.hooks.Deliver(ev, t); !keep {
			return core.Delivery{}, false
		}
	}

	d := core.Delivery{Dest: dst, Event: ev, Parent: parent}

	if ev.Kind == core.KindPost && origKind != core.KindPost {
		d.Fallback = true
	}

	switch ev.Kind {
	case core.KindPost:
		if parentOK && caps.Has(core.CapReply) {
			d.ParentIDs = parentIDs
		}
		return d, true

	case core.KindEdit, core.KindDelete:
		need := core.CapEdit
		if ev.Kind == core.KindDelete {
			need = core.CapDelete
		}
		if !caps.Has(need) {
			return core.Delivery{}, false
		}
		natives, ok := g.ledger.Native(ev.ID, dst)
		if !ok {
			return core.Delivery{}, false
		}
		d.IDs = natives
		return d, true

	case core.KindTyping:
		return d, caps.Has(core.CapTyping)
	}
	return core.Delivery{}, false
}
