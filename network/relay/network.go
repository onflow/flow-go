package relay

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
)

type RelayNetwork struct {
	network.Network
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
		Network:        originNetwork,
		destinationNet: destinationNetwork,
		logger:         logger.With().Str("component", "relay_network").Logger(),
		channels:       channels,
	}
}

func (r *RelayNetwork) Register(channel network.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error) {
	if !r.channels.Contains(channel) {
		return r.Network.Register(channel, messageProcessor)
	}

	relayer, err := NewRelayer(r.destinationNet, channel, messageProcessor)

	if err != nil {
		return nil, fmt.Errorf("failed to register relayer on origin network: %w", err)
	}

	conduit, err := r.Network.Register(channel, relayer)

	if err != nil {
		if closeErr := relayer.Close(); closeErr != nil {
			r.logger.Err(closeErr).Msg("failed to close relayer")
		}

		return nil, fmt.Errorf("failed to register relayer on destination network: %w", err)
	}

	return conduit, nil
}
