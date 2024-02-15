package relay

import (
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type RelayNetwork struct {
	originNet      network.EngineRegistry
	destinationNet network.EngineRegistry
	logger         zerolog.Logger
	channels       map[channels.Channel]channels.Channel
}

var _ network.EngineRegistry = (*RelayNetwork)(nil)

func NewRelayNetwork(
	originNetwork network.EngineRegistry,
	destinationNetwork network.EngineRegistry,
	logger zerolog.Logger,
	channels map[channels.Channel]channels.Channel,
) *RelayNetwork {
	return &RelayNetwork{
		originNet:      originNetwork,
		destinationNet: destinationNetwork,
		logger:         logger.With().Str("component", "relay_network").Logger(),
		channels:       channels,
	}
}

func (r *RelayNetwork) Register(channel channels.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error) {
	// Only relay configured channels
	dstChannel, ok := r.channels[channel]
	if !ok {
		return r.originNet.Register(channel, messageProcessor)
	}

	relayer, err := NewRelayer(r.destinationNet, dstChannel, messageProcessor)

	if err != nil {
		return nil, fmt.Errorf("failed to register relayer on origin network: %w", err)
	}

	conduit, err := r.originNet.Register(channel, relayer)

	if err != nil {
		if closeErr := relayer.Close(); closeErr != nil {
			r.logger.Err(closeErr).Msg("failed to close relayer")
		}

		return nil, fmt.Errorf("failed to register relayer on destination network: %w", err)
	}

	return conduit, nil
}

func (r *RelayNetwork) Start(ctx irrecoverable.SignalerContext) {}

func (r *RelayNetwork) Ready() <-chan struct{} {
	return util.AllReady(r.originNet, r.destinationNet)
}

func (r *RelayNetwork) Done() <-chan struct{} {
	return util.AllDone(r.originNet, r.destinationNet)
}

func (r *RelayNetwork) RegisterBlobService(channel channels.Channel, store datastore.Batching, opts ...network.BlobServiceOption) (network.BlobService, error) {
	return r.originNet.RegisterBlobService(channel, store, opts...)
}

func (r *RelayNetwork) RegisterPingService(pid protocol.ID, provider network.PingInfoProvider) (network.PingService, error) {
	return r.originNet.RegisterPingService(pid, provider)
}
