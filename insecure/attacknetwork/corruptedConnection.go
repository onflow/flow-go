package attacknetwork

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// CorruptedNodeConnection abstracts connection between orchestrator to corrupted conduit factory through the attack network.
type CorruptedNodeConnection struct {
	component.Component
	cm             *component.ComponentManager
	logger         zerolog.Logger
	inboundHandler func(*insecure.Message)                                         // handler for incoming messages from corrupted conduit factory.
	outbound       insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient // from orchestrator to corrupted conduit factory
	inbound        insecure.CorruptibleConduitFactory_RegisterAttackerClient       // from corrupted conduit factory to orchestrator
}

func NewCorruptedNodeConnection(
	logger zerolog.Logger,
	inboundHandler func(message *insecure.Message),
	outbound insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient,
	inbound insecure.CorruptibleConduitFactory_RegisterAttackerClient) *CorruptedNodeConnection {
	c := &CorruptedNodeConnection{
		logger:         logger,
		outbound:       outbound,
		inbound:        inbound,
		inboundHandler: inboundHandler,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			c.receiveLoop()

			ready()

			<-ctx.Done()

		}).Build()

	c.Component = cm
	c.cm = cm

	return c
}

// SendMessage sends the message from orchestrator to the corrupted conduit factory.
func (c *CorruptedNodeConnection) SendMessage(message *insecure.Message) error {
	err := c.outbound.Send(message)
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}

	return nil
}

func (c *CorruptedNodeConnection) receiveLoop() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		for {
			select {
			case <-c.cm.ShutdownSignal():
				// connection closed
				return
			default:
				msg, err := c.inbound.Recv()
				if err == io.EOF || errors.Is(c.inbound.Context().Err(), context.Canceled) {
					c.logger.Warn().Msg("inbound stream closed")
					return
				} else if err != nil {
					c.logger.Error().Err(err).Msg("error reading inbound stream")
				}
				c.inboundHandler(msg)
			}
		}
	}()

	wg.Wait()
}

// CloseConnection closes the connection to the corrupted conduit factory.
func (c *CorruptedNodeConnection) CloseConnection() error {
	err := c.outbound.CloseSend()
	if err != nil {
		return fmt.Errorf("could not close outbound connection: %w", err)
	}

	return nil
}
