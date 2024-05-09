package relay

import (
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type Relayer struct {
	destinationConduit network.Conduit
	messageProcessor   network.MessageProcessor
}

// TODO: currently, any messages received from the destination network on the relay channel will be
// ignored. If a usecase arises, we should implement a mechanism to forward these messages to a handler.
type noopProcessor struct{}

func (n *noopProcessor) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	return nil
}

var _ network.MessageProcessor = (*Relayer)(nil)

func NewRelayer(destinationNetwork network.EngineRegistry, channel channels.Channel, processor network.MessageProcessor) (*Relayer, error) {
	conduit, err := destinationNetwork.Register(channel, &noopProcessor{})

	if err != nil {
		return nil, err
	}

	return &Relayer{
		messageProcessor:   processor,
		destinationConduit: conduit,
	}, nil

}

func (r *Relayer) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	g := new(errgroup.Group)

	g.Go(func() error {
		if err := r.messageProcessor.Process(channel, originID, event); err != nil {
			return fmt.Errorf("failed to relay message to message processor: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		if err := r.destinationConduit.Publish(event, flow.ZeroID); err != nil {
			return fmt.Errorf("failed to relay message to network: %w", err)
		}

		return nil
	})

	return g.Wait()
}

func (r *Relayer) Close() error {
	return r.destinationConduit.Close()
}
