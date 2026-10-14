package gateway

import (
	"context"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
)

func (r *Router) worker(ctx context.Context, dst core.Address, b core.Bridge, q <-chan core.Event) {
	var dropped int
	for ev := range q {
		if ctx.Err() != nil {
			dropped++
			continue
		}
		r.deliver(ctx, dst, b, ev)
	}
	if dropped > 0 {
		r.log.Warn("shutdown timed out, dropped queued events", "dst", dst, "count", dropped)
	}
}

func (r *Router) deliver(ctx context.Context, dst core.Address, b core.Bridge, ev core.Event) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	caps := b.Caps() // re-run every time as caps may change (e.g. IRC)
	ev = r.resolveFiles(ctx, ev, dst, caps)
	d, ok := r.lower(ev, dst, caps)
	if !ok {
		return
	}

	receipt, err := b.Deliver(ctx, d)
	if err != nil {
		r.log.Warn("deliver", "dst", dst, "err", err)
		return
	}
	if len(receipt.IDs) == 0 {
		return
	}
	if d.Fallback {
		existing, _ := r.ledger.Native(ev.ID, dst)
		r.ledger.Bind(ev.ID, dst, append(existing, receipt.IDs...))
	} else {
		r.ledger.Bind(ev.ID, dst, receipt.IDs)
	}
}

func (r *Router) resolveFiles(ctx context.Context, ev core.Event, dst core.Address, caps core.CapSet) core.Event {
	// platforms with CapMedia use .Bytes() so they won't ever need
	// the media proxy
	if caps.Has(core.CapMedia) || len(ev.Files) == 0 {
		return ev
	}

	out := make([]core.File, 0, len(ev.Files))
	for _, f := range ev.Files {
		url, err := r.media.URL(ctx, f)
		if err != nil {
			r.log.Warn("media resolve", "dst", dst, "file", f.Name, "err", err)
			continue
		}
		if url == "" {
			r.log.Warn("media resolve: empty url, dropping file", "dst", dst, "file", f.Name)
			continue
		}
		f.URL, f.Public, f.Data, f.Key = url, true, nil, ""
		out = append(out, f)
	}

	ev.Files = out
	return ev
}

func (r *Router) lower(ev core.Event, dst core.Address, caps core.CapSet) (core.Delivery, bool) {
	origKind := ev.Kind

	var parent *core.Message
	var parentIDs []string
	var parentOK bool
	if ev.ParentID != "" {
		if rec, ok := r.ledger.Get(ev.ParentID); ok {
			parent = &rec
		}
		parentIDs, parentOK = r.ledger.Native(ev.ParentID, dst)
	}

	if r.hooks != nil {
		t := hook.Target{Platform: r.accounts[dst.Account].Platform, Address: dst, Caps: caps, Parent: parent}
		if ev.ParentID != "" && !parentOK {
			t.Caps = caps.Without(core.CapReply) // has parent id but can't resolve
		}
		var keep bool
		if ev, keep = r.hooks.Deliver(ev, t); !keep {
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
		natives, ok := r.ledger.Native(ev.ID, dst)
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
