package core

type ChannelSpec struct {
	Name string
	Key  string
}

type StatusEvent uint8

const (
	Ready        StatusEvent = iota // account connected, ready to receive Join() calls
	Disconnected                    // session dead, gateway re-Opens
	ChannelUp                       // confirmed joined a channel
	ChannelDown                     // not in a desired channel
)

type Status struct {
	Event   StatusEvent
	Channel string // for ChannelUp / ChannelDown
	Err     error  // for ChannelDown, ErrPermanent gives up
}
