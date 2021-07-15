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
	subMngr   network.SubscriptionManager
}

func NewNetwork(
	net module.Network,
	log zerolog.Logger,
	subMngr network.SubscriptionManager,
) (*Network, error) {
	e := &Network{
		net:       net,
		splitters: make(map[network.Channel]*splitterEngine.Engine),
		conduits:  make(map[network.Channel]network.Conduit),
		log:       log,
		subMngr:   subMngr,
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

	splitterEng, err := n.subMngr.GetEngine(channel)

	if err != nil {
		// create new splitter for the channel
		splitterEng = splitterEngine.New(
			n.log,
			channel,
		)

		// register it with subscription manager
		if err = n.subMngr.Register(channel, splitterEng); err != nil {
			return nil, fmt.Errorf("failed to register splitter for channel %s: %w", channel, err)
		}
	}

	splitter, ok := splitterEng.(*splitterEngine.Engine)

	if !ok {
		return nil, errors.New("got unexpected engine type from subscription manager")
	}

	// register engine with splitter
	if err = splitter.RegisterEngine(engine); err != nil {
		n.subMngr.Unregister(channel)
		return nil, fmt.Errorf("failed to register engine with splitter: %w", err)
	}

	conduit, ok := n.conduits[channel]

	if !ok {
		conduit, err = n.net.Register(channel, splitter)

		if err != nil {
			// undo previous steps
			splitter.UnregisterEngine(engine)
			n.subMngr.Unregister(channel)

			return nil, fmt.Errorf("failed to register splitter engine on channel %s: %w", channel, err)
		}

		n.conduits[channel] = conduit
	}

	return conduit, nil
}
