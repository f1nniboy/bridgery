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
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	if err := r.relay(ctx, in); err != nil {
		r.log.Warn("ledger", "origin", in.Origin, "kind", in.Kind, "err", err)
	}
}

func (r *Router) relay(ctx context.Context, in core.Inbound) error {
	ev, ok, err := r.lift(ctx, in)
	if !ok || err != nil {
		return err
	}
	if r.hooks != nil {
		if ev, ok = r.hooks.Receive(ev); !ok {
			return nil // cancelled by hook
		}
	}
	if ev.Kind == core.KindEdit {
		rec, found, err := r.ledger.Get(ctx, ev.ID)
		if err != nil {
			return err
		}
		if found && slices.Equal(rec.Content, ev.Content) {
			return nil // edit with same content as before, so skip
		}
	}
	if ev.Kind == core.KindPost || ev.Kind == core.KindEdit {
		if err := r.ledger.Put(ctx, ev.ID, ev.Message); err != nil {
			return err
		}
	}
	for _, dst := range r.routes[ev.Origin] {
		select {
		case r.queues[dst] <- ev:
		default:
			r.log.Warn("queue full, dropping event", "dst", dst, "kind", ev.Kind)
		}
	}
	return nil
}

func (r *Router) lift(ctx context.Context, in core.Inbound) (core.Event, bool, error) {
	ev := core.Event{Kind: in.Kind, Origin: in.Origin, Platform: r.accounts[in.Origin.Account].Platform, Message: in.Message}

	switch in.Kind {
	case core.KindPost:
		if in.NativeID != "" {
			_, echo, err := r.ledger.Canonical(ctx, in.Origin, in.NativeID)
			if echo || err != nil {
				return core.Event{}, false, err // already seen, bridges drop our own echoes before this
			}
		}
		ev.ID = core.NewID()
		if in.NativeID != "" {
			if err := r.ledger.Bind(ctx, ev.ID, in.Origin, []string{in.NativeID}); err != nil {
				return core.Event{}, false, err
			}
		}

	case core.KindEdit:
		id, ok, err := r.ledger.Canonical(ctx, in.Origin, in.NativeID)
		if !ok || err != nil {
			return core.Event{}, false, err
		}
		ev.ID = id

	case core.KindDelete:
		id, ok, err := r.ledger.Canonical(ctx, in.Origin, in.NativeID)
		if !ok || err != nil {
			return core.Event{}, false, err
		}
		rec, found, err := r.ledger.Get(ctx, id)
		if !found || err != nil {
			return core.Event{}, false, err // already applied, e.g. the echo of a delete we relayed
		}
		ev.ID = id
		ev.Author = rec.Author
		ev.Content, ev.Files = rec.Content, rec.Files // carry what was deleted for hooks
		if err := r.ledger.Delete(ctx, id); err != nil {
			return core.Event{}, false, err
		}
	}

	if in.ParentID != "" {
		id, ok, err := r.ledger.Canonical(ctx, in.Origin, in.ParentID)
		if err != nil {
			return core.Event{}, false, err
		}
		if ok {
			ev.ParentID = id
		}
	}

	return ev, true, nil
}
