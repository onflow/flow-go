package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// CorruptedNodeConnection abstracts connection between an attack orchestrator to a corruptible conduit factory (ccf)
// through the orchestrator network.
type CorruptedNodeConnection struct {
	component.Component
	cm             *component.ComponentManager
	logger         zerolog.Logger
	inboundHandler func(*insecure.Message)                              // handler for incoming messages from corruptible conduit factories.
	outbound       insecure.CorruptNetwork_ProcessAttackerMessageClient // from attacker to corrupt network.
	inbound        insecure.CorruptNetwork_ConnectAttackerClient        // from corrupt network to attacker.
}

func NewCorruptedNodeConnection(
	logger zerolog.Logger,
	inboundHandler func(message *insecure.Message),
	outbound insecure.CorruptNetwork_ProcessAttackerMessageClient,
	inbound insecure.CorruptNetwork_ConnectAttackerClient) *CorruptedNodeConnection {
	c := &CorruptedNodeConnection{
		logger:         logger.With().Str("component", "corrupted-connector").Logger(),
		outbound:       outbound,
		inbound:        inbound,
		inboundHandler: inboundHandler,
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {

			ready()
			c.receiveLoop()

			<-ctx.Done()

		}).Build()

	c.Component = cm
	c.cm = cm

	return c
}

// SendMessage sends the message from orchestrator to the corrupted conduit factory.
func (c *CorruptedNodeConnection) SendMessage(message *insecure.Message) error {
	err := c.outbound.Send(message)
	if err == io.EOF {
		c.logger.Warn().Err(err).Msg("outbound stream to corrupt node is closed")
	}
	if err != nil {
		return fmt.Errorf("could not send message: %w", err)
	}

	return nil
}

// receiveLoop implements the continuous procedure of reading from inbound stream of this connection, which
// is established from the remote ccf to the local attack orchestrator.
func (c *CorruptedNodeConnection) receiveLoop() {
	c.logger.Info().Msg("receive loop started")

	for {
		select {
		case <-c.cm.ShutdownSignal():
			// connection closed
			c.logger.Info().Msg("receive loop terminated")
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
}

// CloseConnection closes the connection to the corrupted conduit factory.
func (c *CorruptedNodeConnection) CloseConnection() error {
	err := c.outbound.CloseSend()
	if err != nil {
		return fmt.Errorf("could not close outbound connection: %w", err)
	}

	return nil
}
