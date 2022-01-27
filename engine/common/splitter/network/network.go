package network

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	splitterEngine "github.com/onflow/flow-go/engine/common/splitter"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
)

// Network is the splitter network. It is a wrapper around the default network implementation
// and should be passed in to engine constructors that require a network to register with.
// When an engine is registered with the splitter network, a splitter engine is created for
// the given channel (if one doesn't already exist) and the engine is registered with that
// splitter engine. As a result, multiple engines can register with the splitter network on
// the same channel and will each receive all events on that channel.
type Network struct {
	net       network.Network
	mu        sync.RWMutex
	log       zerolog.Logger
	splitters map[network.Channel]*splitterEngine.Engine // stores splitters for each channel
	conduits  map[network.Channel]network.Conduit        // stores conduits for all registered channels
	*component.ComponentManager
}

var _ network.Network = (*Network)(nil)

// NewNetwork returns a new splitter network.
func NewNetwork(
	net network.Network,
	log zerolog.Logger,
) *Network {
	n := &Network{
		net:       net,
		splitters: make(map[network.Channel]*splitterEngine.Engine),
		conduits:  make(map[network.Channel]network.Conduit),
		log:       log,
	}

	n.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			err := util.WaitClosed(ctx, n.net.Ready())

			if err != nil {
				return
			}

			ready()

			<-ctx.Done()
		}).Build()

	return n
}

func (n *Network) RegisterBlobService(channel network.Channel, store datastore.Batching) (network.BlobService, error) {
	return n.net.RegisterBlobService(channel, store)
}

func (n *Network) RegisterPingService(pid protocol.ID, provider network.PingInfoProvider) (network.PingService, error) {
	return n.net.RegisterPingService(pid, provider)
}

// Register will subscribe the given engine with the spitter on the given channel, and all registered
// engines will be notified with incoming messages on the channel.
// The returned Conduit can be used to send messages to engines on other nodes subscribed to the same channel
func (n *Network) Register(channel network.Channel, engine network.MessageProcessor) (network.Conduit, error) {
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
