package core

type Address struct {
	Account string // name in config
	Channel string // native channel/room ID
}

func (a Address) String() string {
	if a.Channel == "" {
		return a.Account
	}
	return a.Account + "/" + a.Channel
}
