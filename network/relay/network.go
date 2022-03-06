package relay

import (
	"fmt"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
)

type RelayNetwork struct {
	originNet      network.Network
	destinationNet network.Network
	logger         zerolog.Logger
	channels       network.ChannelList
}

var _ network.Network = (*RelayNetwork)(nil)

func NewRelayNetwork(
	originNetwork network.Network,
	destinationNetwork network.Network,
	logger zerolog.Logger,
	channels []network.Channel,
) *RelayNetwork {
	return &RelayNetwork{
		originNet:      originNetwork,
		destinationNet: destinationNetwork,
		logger:         logger.With().Str("component", "relay_network").Logger(),
		channels:       channels,
	}
}

func (r *RelayNetwork) RegisterDirectMessageHandler(channel network.Channel, handler network.DirectMessageHandler) error {
	// TODO: for now, we have no usecase that requires relaying direct messages.
	return r.originNet.RegisterDirectMessageHandler(channel, handler)
}

func (r *RelayNetwork) SetDirectMessageConfig(channel network.Channel, config network.DirectMessageConfig) error {
	return r.originNet.SetDirectMessageConfig(channel, config)
}

func (r *RelayNetwork) SendDirectMessage(channel network.Channel, message interface{}, target flow.Identifier) error {
	return r.originNet.SendDirectMessage(channel, message, target)
}

func (r *RelayNetwork) Register(channel network.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error) {
	if !r.channels.Contains(channel) {
		return r.originNet.Register(channel, messageProcessor)
	}

	relayer, err := NewRelayer(r.destinationNet, channel, messageProcessor)

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

func (r *RelayNetwork) RegisterBlobService(channel network.Channel, store datastore.Batching, opts ...network.BlobServiceOption) (network.BlobService, error) {
	return r.originNet.RegisterBlobService(channel, store, opts...)
}

func (r *RelayNetwork) RegisterPingService(pid protocol.ID, provider network.PingInfoProvider) (network.PingService, error) {
	return r.originNet.RegisterPingService(pid, provider)
}
