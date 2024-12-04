package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

type Controller struct {
	logger zerolog.Logger
	config Config
	conn   WebsocketConnection

	// data channel which data providers write messages to.
	// writer routine reads from this channel and writes messages to connection
	multiplexedStream chan interface{}

	dataProviders       *concurrentmap.Map[uuid.UUID, dp.DataProvider]
	dataProviderFactory dp.DataProviderFactory
}

func NewWebSocketController(
	logger zerolog.Logger,
	config Config,
	conn WebsocketConnection,
	dataProviderFactory dp.DataProviderFactory,
) *Controller {
	return &Controller{
		logger:              logger.With().Str("component", "websocket-controller").Logger(),
		config:              config,
		conn:                conn,
		multiplexedStream:   make(chan interface{}),
		dataProviders:       concurrentmap.New[uuid.UUID, dp.DataProvider](),
		dataProviderFactory: dataProviderFactory,
	}
}

// HandleConnection manages the lifecycle of a WebSocket connection,
// including setup, message processing, and graceful shutdown.
//
// Parameters:
// - ctx: The context for controlling cancellation and timeouts.
func (c *Controller) HandleConnection(ctx context.Context) {
	defer c.shutdownConnection()

	err := c.configureKeepalive()
	if err != nil {
		c.logger.Error().Err(err).Msg("error configuring connection")
		return
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return c.readMessages(gCtx)
	})
	g.Go(func() error {
		return c.keepalive(gCtx)
	})
	g.Go(func() error {
		return c.writeMessages(gCtx)
	})

	if err = g.Wait(); err != nil {
		c.logger.Error().Err(err).Msg("error detected in one of the goroutines")
	}
}

// configureKeepalive sets up the WebSocket connection with a read deadline
// and a handler for receiving pong messages from the client.
//
// The function does the following:
//  1. Sets an initial read deadline to ensure the server doesn't wait indefinitely
//     for a pong message from the client. If no message is received within the
//     specified `pongWait` duration, the connection will be closed.
//  2. Establishes a Pong handler that resets the read deadline every time a pong
//     message is received from the client, allowing the server to continue waiting
//     for further pong messages within the new deadline.
//
// No errors are expected during normal operation.
func (c *Controller) configureKeepalive() error {
	// Set the initial read deadline for the first pong message
	// The Pong handler itself only resets the read deadline after receiving a Pong.
	// It doesn't set an initial deadline. The initial read deadline is crucial to prevent the server from waiting
	// forever if the client doesn't send Pongs.
	if err := c.conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return fmt.Errorf("failed to set the initial read deadline: %w", err)
	}

	// Establish a Pong handler which sets the handler for pong messages received from the peer.
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	return nil
}

// writeMessages reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
//
// Expected errors during normal operation:
// - context.Canceled if the client disconnected
func (c *Controller) writeMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-c.multiplexedStream:
			if !ok {
				return nil
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return fmt.Errorf("failed to set the write deadline: %w", err)
			}

			err := c.conn.WriteJSON(msg)
			if err != nil {
				if IsCloseError(err) {
					return nil
				}
				c.logger.Error().Err(err).Msg("failed to write msg to connection")
			}
		}
	}
}

// readMessages continuously reads messages from a client WebSocket connection,
// processes each message, and handles actions based on the message type.
//
// Expected errors during normal operation:
// - context.Canceled if the client disconnected
func (c *Controller) readMessages(ctx context.Context) error {
	for {
		msg, err := c.readMessage()
		if err != nil {
			if IsCloseError(err) {
				return nil
			}

			c.logger.Error().Err(err).Msg("error reading message")
			continue
		}

		validatedMsg, err := c.parseAndValidateMessage(msg)
		if err != nil {
			c.logger.Error().Err(err).Msg("failed to parse message")
			continue
		}

		if err := c.handleAction(ctx, validatedMsg); err != nil {
			c.logger.Error().Err(err).Msg("failed to handle action")
			continue
		}
	}
}

func (c *Controller) readMessage() (json.RawMessage, error) {
	var message json.RawMessage
	if err := c.conn.ReadJSON(&message); err != nil {
		return nil, fmt.Errorf("error reading JSON from client: %w", err)
	}
	return message, nil
}

func (c *Controller) parseAndValidateMessage(message json.RawMessage) (interface{}, error) {
	var baseMsg models.BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return nil, fmt.Errorf("error unmarshalling base message: %w", err)
	}

	var validatedMsg interface{}
	switch baseMsg.Action {
	case "subscribe":
		var subscribeMsg models.SubscribeMessageRequest
		if err := json.Unmarshal(message, &subscribeMsg); err != nil {
			return nil, fmt.Errorf("error unmarshalling subscribe message: %w", err)
		}
		//TODO: add validation logic for `topic` field
		validatedMsg = subscribeMsg

	case "unsubscribe":
		var unsubscribeMsg models.UnsubscribeMessageRequest
		if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
			return nil, fmt.Errorf("error unmarshalling unsubscribe message: %w", err)
		}
		validatedMsg = unsubscribeMsg

	case "list_subscriptions":
		var listMsg models.ListSubscriptionsMessageRequest
		if err := json.Unmarshal(message, &listMsg); err != nil {
			return nil, fmt.Errorf("error unmarshalling list subscriptions message: %w", err)
		}
		validatedMsg = listMsg

	default:
		c.logger.Debug().Str("action", baseMsg.Action).Msg("unknown action type")
		return nil, fmt.Errorf("unknown action type: %s", baseMsg.Action)
	}

	return validatedMsg, nil
}

func (c *Controller) handleAction(ctx context.Context, message interface{}) error {
	switch msg := message.(type) {
	case models.SubscribeMessageRequest:
		c.handleSubscribe(ctx, msg)
	case models.UnsubscribeMessageRequest:
		c.handleUnsubscribe(ctx, msg)
	case models.ListSubscriptionsMessageRequest:
		c.handleListSubscriptions(ctx, msg)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
	return nil
}

func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	dp, err := c.dataProviderFactory.NewDataProvider(ctx, msg.Topic, msg.Arguments, c.multiplexedStream)
	if err != nil {
		// TODO: handle error here
		c.logger.Error().Err(err).Msgf("error while creating data provider for topic: %s", msg.Topic)
	}

	c.dataProviders.Add(dp.ID(), dp)

	//TODO: return OK response to client
	c.multiplexedStream <- msg

	go func() {
		err := dp.Run()
		if err != nil {
			//TODO: Log or handle the error from Run
			c.logger.Error().Err(err).Msgf("error while running data provider for topic: %s", msg.Topic)
		}
	}()
}

func (c *Controller) handleUnsubscribe(_ context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.logger.Debug().Err(err).Msg("error parsing message ID")
		//TODO: return an error response to client
		c.multiplexedStream <- err
		return
	}

	dp, ok := c.dataProviders.Get(id)
	if ok {
		dp.Close()
		c.dataProviders.Remove(id)
	}
}

func (c *Controller) handleListSubscriptions(ctx context.Context, msg models.ListSubscriptionsMessageRequest) {
	//TODO: return a response to client
}

func (c *Controller) shutdownConnection() {
	defer func() {
		if err := c.conn.Close(); err != nil {
			c.logger.Error().Err(err).Msg("error closing connection")
		}
	}()

	err := c.dataProviders.ForEach(func(_ uuid.UUID, dp dp.DataProvider) error {
		dp.Close()
		return nil
	})
	if err != nil {
		c.logger.Error().Err(err).Msg("error closing data provider")
	}

	c.dataProviders.Clear()
}

// keepalive sends a ping message periodically to keep the WebSocket connection alive
// and avoid timeouts.
//
// Expected errors during normal operation:
// - context.Canceled if the client disconnected
func (c *Controller) keepalive(ctx context.Context) error {
	pingTicker := time.NewTicker(PingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pingTicker.C:
			err := c.conn.WriteControl(websocket.PingMessage, time.Now().Add(WriteWait))
			if err != nil {
				// Log error and exit the loop on failure
				c.logger.Debug().Err(err).Msg("failed to send ping")

				return fmt.Errorf("failed to write ping message: %w", err)
			}
		}
	}
}

func IsCloseError(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure, websocket.CloseGoingAway)
}
