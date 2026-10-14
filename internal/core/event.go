package core

type Kind string

const (
	KindPost   Kind = "post"
	KindEdit   Kind = "edit"
	KindDelete Kind = "delete"
	KindTyping Kind = "typing"
)

// a message as a platform emits it, carrying native IDs
type Inbound struct {
	Kind     Kind
	Origin   Address
	NativeID string // this message's own native ID, empty for ephemeral kinds
	ParentID string // native ID of the referenced message
	Message
}

// canonical form of an inbound message (after transforming from core.Inbound)
type Event struct {
	Kind     Kind    `luar:"kind"`
	ID       CID     `luar:"-"`
	Origin   Address `luar:"-"`
	Platform string  `luar:"platform"`
	ParentID CID     `luar:"-"`
	Message
}
