package core

type ChannelSpec struct {
	Name string
	Key  string
}

type StatusEvent uint8

const (
	Disconnected StatusEvent = iota // session dead, gateway re-Opens
	ChannelLost                     // removed from channel, gateway re-Joins
)

type Status struct {
	Event   StatusEvent
	Channel string // for ChannelLost
	Err     error  // for ChannelLost, ErrPermanent gives up
}
