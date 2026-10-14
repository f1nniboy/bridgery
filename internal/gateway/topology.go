package gateway

import (
	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
)

func buildRoutes(defs []config.Gateway) map[core.Address][]core.Address {
	routes := make(map[core.Address][]core.Address)
	seen := make(map[[2]core.Address]bool)
	for _, gw := range defs {
		for _, from := range gw.Channels {
			for _, to := range gw.Channels {
				src, dst := from.Address(), to.Address()
				pair := [2]core.Address{src, dst}
				if src == dst || !from.Sources() || !to.Receives() || seen[pair] {
					continue
				}
				seen[pair] = true
				routes[src] = append(routes[src], dst)
			}
		}
	}
	return routes
}

func buildChannels(defs []config.Gateway) map[string][]core.ChannelSpec {
	channels := make(map[string][]core.ChannelSpec)
	seen := make(map[core.Address]bool)
	for _, gw := range defs {
		for _, c := range gw.Channels {
			if seen[c.Address()] {
				continue
			}
			seen[c.Address()] = true
			channels[c.Account] = append(channels[c.Account], core.ChannelSpec{Name: c.Channel, Key: string(c.Key)})
		}
	}
	return channels
}

func buildQueues(routes map[core.Address][]core.Address, size int) map[core.Address]chan core.Event {
	queues := make(map[core.Address]chan core.Event)
	for _, dests := range routes {
		for _, dst := range dests {
			if _, ok := queues[dst]; !ok {
				queues[dst] = make(chan core.Event, size)
			}
		}
	}
	return queues
}
