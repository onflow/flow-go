package network

import (
	"errors"
	"fmt"
	"sync"

	splitterEngine "github.com/onflow/flow-go/engine/common/splitter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/rs/zerolog"
)

type Network struct {
	mu        sync.RWMutex
	net       module.Network
	log       zerolog.Logger
	splitters map[network.Channel]*splitterEngine.Engine // stores splitters for each channel
	conduits  map[network.Channel]network.Conduit        // stores conduits for all registered channels
}

func NewNetwork(
	net module.Network,
	log zerolog.Logger,
) (*Network, error) {
	e := &Network{
		net:       net,
		splitters: make(map[network.Channel]*splitterEngine.Engine),
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

	splitter, splitterExists := n.splitters[channel]
	conduit, conduitExists := n.conduits[channel]

	if splitterExists != conduitExists {
		return nil, errors.New("inconsistent state detected")
	}

	channelRegistered := splitterExists && conduitExists

	if !channelRegistered {
		// create new splitter for the channel
		splitter = splitterEngine.New(
			n.log,
			channel,
		)

		n.splitters[channel] = splitter
	}

	// register engine with splitter
	err := splitter.RegisterEngine(engine)

	if err != nil {
		// remove the splitter engine if this was the first time the given channel was registered
		if !channelRegistered {
			delete(n.splitters, channel)
		}

		return nil, fmt.Errorf("failed to register engine with splitter: %w", err)
	}

	if !channelRegistered {
		conduit, err = n.net.Register(channel, splitter)

		if err != nil {
			// undo previous steps
			splitter.UnregisterEngine(engine)
			delete(n.splitters, channel)

			return nil, fmt.Errorf("failed to register splitter engine on channel %s: %w", channel, err)
		}

		n.conduits[channel] = conduit
	}

	return conduit, nil
}
