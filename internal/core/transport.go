package core

import (
	"context"
	"errors"
)

var ErrPermanent = errors.New("permanent failure")

type CapSet map[string]bool

const (
	CapReply  = "reply"
	CapEdit   = "edit"
	CapDelete = "delete"
	CapTyping = "typing"
	CapPuppet = "puppet"
	CapMedia  = "media" // the platform can store media itself (e.g. CDN)
)

var AllCaps = []string{CapReply, CapEdit, CapDelete, CapTyping, CapPuppet, CapMedia}

func Caps(names ...string) CapSet {
	c := make(CapSet, len(names))
	for _, n := range names {
		c[n] = true
	}
	return c
}

func (c CapSet) Has(name string) bool { return c[name] }

func (c CapSet) Without(name string) CapSet {
	out := make(CapSet, len(c))
	for k := range c {
		if k != name {
			out[k] = true
		}
	}
	return out
}

type Delivery struct {
	Dest      Address
	Event     Event
	IDs       []string
	Parent    *Message // the referenced message, when the event has a parent
	ParentIDs []string
	Fallback  bool // rewritten into a post by a hook, receipt is bound additively
}

type Receipt struct {
	IDs []string
}

type Bridge interface {
	Name() string
	Example() Bridge
	Caps() CapSet

	Open(ctx context.Context) error                      // ctx lives for the session, cancelled before Close
	Join(ctx context.Context, channel ChannelSpec) error // returns once joined, ErrPermanent if it never will be
	Close() error

	Deliver(ctx context.Context, d Delivery) (Receipt, error)

	Status() <-chan Status
	Inbound() <-chan Inbound
}
