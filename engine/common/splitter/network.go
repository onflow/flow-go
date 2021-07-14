package splitter

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
)

type Network struct {
	mu        sync.RWMutex
	net       module.Network
	log       zerolog.Logger
	splitters map[network.Channel]*Engine         // stores splitters for each channel
	conduits  map[network.Channel]network.Conduit // stores conduits for all registered channels
}

func NewNetwork(
	net module.Network,
	log zerolog.Logger,
) (*Network, error) {
	e := &Network{
		net:       net,
		splitters: make(map[network.Channel]*Engine),
		conduits:  make(map[network.Channel]network.Conduit),
		log:       log,
	}

	return e, nil
}

// Register will subscribe the given engine with the spitter on the given channel, and all registered
// engines will be notified with incoming messages on the channel.
// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
func (n *Network) Register(channel network.Channel, e network.Engine) (network.Conduit, error) {
	engine, ok := e.(module.Engine)

	if !ok {
		return nil, errors.New("engine does not have the correct type")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	splitter, ok := n.splitters[channel]

	if !ok {
		splitter := New(
			n.log,
			channel,
		)

		n.splitters[channel] = splitter
	}

	if err := splitter.RegisterEngine(engine); err != nil {
		return nil, fmt.Errorf("failed to register engine with splitter: %w", err)
	}

	conduit, ok := n.conduits[channel]

	if !ok {
		conduit, err := n.net.Register(channel, splitter)

		if err != nil {
			splitter.UnregisterEngine(engine)
			delete(n.splitters, channel)
			return nil, fmt.Errorf("failed to register splitter engine on channel %s: %w", channel, err)
		}

		n.conduits[channel] = conduit
	}

	return conduit, nil
}

// Channels returns all the channels registered on this network.
func (n *Network) Channels() network.ChannelList {
	n.mu.RLock()
	defer n.mu.RUnlock()

	channels := make(network.ChannelList, 0)
	for channel := range n.conduits {
		channels = append(channels, channel)
	}

	return channels
}
