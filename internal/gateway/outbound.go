package gateway

import (
	"context"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
)

func (r *Router) worker(ctx context.Context, dst core.Address, q <-chan core.Event) {
	var dropped int
	for ev := range q {
		if ctx.Err() != nil {
			dropped++
			continue
		}
		r.deliver(ctx, dst, ev)
	}
	if dropped > 0 {
		r.log.Warn("shutdown timed out, dropped queued events", "dst", dst, "count", dropped)
	}
}

func (r *Router) deliver(ctx context.Context, dst core.Address, ev core.Event) {
	acc := r.accounts[dst.Account]
	if !acc.acquire() {
		r.log.Warn("account not connected, dropping event", "dst", dst, "kind", ev.Kind)
		return
	}
	defer acc.release()

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	caps := acc.Bridge.Caps() // re-run every time as caps may change (e.g. IRC)
	ev = r.resolveFiles(ctx, ev, dst, caps)
	d, ok, err := r.lower(ctx, ev, dst, caps)
	if err != nil {
		r.log.Warn("ledger", "dst", dst, "kind", ev.Kind, "err", err)
		return
	}
	if !ok {
		return
	}

	receipt, err := acc.Bridge.Deliver(ctx, d)
	if err != nil {
		r.log.Warn("deliver", "dst", dst, "err", err)
		return
	}
	if len(receipt.IDs) == 0 {
		return
	}

	bindCtx, cancelBind := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
	defer cancelBind()
	ids := receipt.IDs
	if d.Fallback {
		existing, _, err := r.ledger.Native(bindCtx, ev.ID, dst)
		if err != nil {
			r.log.Warn("ledger", "dst", dst, "err", err)
			return
		}
		ids = append(existing, ids...)
	}
	if err := r.ledger.Bind(bindCtx, ev.ID, dst, ids); err != nil {
		r.log.Warn("ledger", "dst", dst, "err", err)
	}
}

func (r *Router) resolveFiles(ctx context.Context, ev core.Event, dst core.Address, caps core.CapSet) core.Event {
	// platforms with CapMedia use .Bytes() so they don't need the media proxy
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
		f.URL, f.Public, f.Data, f.Fetch, f.Key = url, true, nil, nil, ""
		out = append(out, f)
	}

	ev.Files = out
	return ev
}

func (r *Router) lower(ctx context.Context, ev core.Event, dst core.Address, caps core.CapSet) (core.Delivery, bool, error) {
	origKind := ev.Kind

	var parent *core.Message
	var parentIDs []string
	var parentOK bool
	if ev.ParentID != "" {
		rec, found, err := r.ledger.Get(ctx, ev.ParentID)
		if err != nil {
			return core.Delivery{}, false, err
		}
		if found {
			parent = &rec
		}
		if parentIDs, parentOK, err = r.ledger.Native(ctx, ev.ParentID, dst); err != nil {
			return core.Delivery{}, false, err
		}
	}

	if r.hooks != nil {
		t := hook.Target{Platform: r.accounts[dst.Account].Platform, Address: dst, Caps: caps, Parent: parent}
		if ev.ParentID != "" && !parentOK {
			t.Caps = caps.Without(core.CapReply) // has parent id but can't resolve
		}
		var keep bool
		if ev, keep = r.hooks.Deliver(ev, t); !keep {
			return core.Delivery{}, false, nil
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
		return d, true, nil

	case core.KindEdit, core.KindDelete:
		need := core.CapEdit
		if ev.Kind == core.KindDelete {
			need = core.CapDelete
		}
		if !caps.Has(need) {
			return core.Delivery{}, false, nil
		}
		natives, ok, err := r.ledger.Native(ctx, ev.ID, dst)
		if !ok || err != nil {
			return core.Delivery{}, false, err
		}
		d.IDs = natives
		return d, true, nil

	case core.KindTyping:
		return d, caps.Has(core.CapTyping), nil
	}
	return core.Delivery{}, false, nil
}
