package network

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	splitterEngine "github.com/onflow/flow-go/engine/common/splitter"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network"
)

// Network is the splitter network. It is a wrapper around the default network implementation
// and should be passed in to engine constructors that require a network to register with.
// When an engine is registered with the splitter network, a splitter engine is created for
// the given channel (if one doesn't already exist) and the engine is registered with that
// splitter engine. As a result, multiple engines can register with the splitter network on
// the same channel and will each receive all events on that channel.
type Network struct {
	network.Network
	mu        sync.RWMutex
	log       zerolog.Logger
	splitters map[network.Channel]*splitterEngine.Engine // stores splitters for each channel
	conduits  map[network.Channel]network.Conduit        // stores conduits for all registered channels
	lm        *lifecycle.LifecycleManager
}

// NewNetwork returns a new splitter network.
func NewNetwork(
	net network.Network,
	log zerolog.Logger,
) *Network {
	return &Network{
		Network:   net,
		splitters: make(map[network.Channel]*splitterEngine.Engine),
		conduits:  make(map[network.Channel]network.Conduit),
		log:       log,
		lm:        lifecycle.NewLifecycleManager(),
	}
}

// Register will subscribe the given engine with the spitter on the given channel, and all registered
// engines will be notified with incoming messages on the channel.
// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
func (n *Network) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
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
	splitter.RegisterEngine(engine)

	if !channelRegistered {
		var err error
		conduit, err = n.Network.Register(channel, splitter)

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

// Ready returns a ready channel that is closed once the network has fully
// started. For the splitter network, this is true once the wrapped network
// has started.
func (n *Network) Ready() <-chan struct{} {
	n.lm.OnStart(func() {
		<-n.Network.Ready()
	})
	return n.lm.Started()
}

// Done returns a done channel that is closed once the network has fully stopped.
// For the splitter network, this is true once the wrapped network has stopped.
func (n *Network) Done() <-chan struct{} {
	n.lm.OnStop(func() {
		<-n.Network.Done()
	})
	return n.lm.Stopped()
}
