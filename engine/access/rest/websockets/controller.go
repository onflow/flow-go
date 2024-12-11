package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
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
	dataProvidersGroup  *sync.WaitGroup
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
		dataProvidersGroup:  &sync.WaitGroup{},
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
		if errors.Is(err, websocket.ErrCloseSent) {
			return
		}

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
		case message, ok := <-c.multiplexedStream:
			if !ok {
				return fmt.Errorf("multiplexed stream closed")
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return fmt.Errorf("failed to set the write deadline: %w", err)
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return err
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
		var message json.RawMessage
		if err := c.conn.ReadJSON(&message); err != nil {
			if errors.Is(err, websocket.ErrCloseSent) {
				return err
			}

			c.writeBaseErrorResponse(ctx, err, "")
			c.logger.Error().Err(err).Msg("error reading message")
			continue
		}

		validatedMsg, err := c.parseAndValidateMessage(message)
		if err != nil {
			c.writeBaseErrorResponse(ctx, err, "")
			c.logger.Error().Err(err).Msg("failed to parse message")
			continue
		}

		if err = c.handleAction(ctx, validatedMsg); err != nil {
			c.writeBaseErrorResponse(ctx, err, "")
			c.logger.Error().Err(err).Msg("failed to handle action")
			continue
		}
	}
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
		c.handleListSubscriptions(ctx)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
	return nil
}

func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	// register new provider
	provider, err := c.dataProviderFactory.NewDataProvider(ctx, msg.Topic, msg.Arguments, c.multiplexedStream)
	if err != nil {
		c.writeBaseErrorResponse(ctx, err, "subscribe")
		c.logger.Error().Err(err).Msg("error creating data provider")
		return
	}

	c.dataProviders.Add(provider.ID(), provider)
	c.writeSubscribeOkResponse(ctx, provider.ID())

	// run provider
	c.dataProvidersGroup.Add(1)
	go func() {
		err = provider.Run()
		if err != nil {
			c.writeBaseErrorResponse(ctx, err, "")
			c.logger.Error().Err(err).Msgf("error while running data provider for topic: %s", msg.Topic)
		}

		c.dataProvidersGroup.Done()
		c.dataProviders.Remove(provider.ID())
	}()
}

func (c *Controller) handleUnsubscribe(ctx context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.writeBaseErrorResponse(ctx, err, "unsubscribe")
		c.logger.Debug().Err(err).Msg("error parsing message ID")
		return
	}

	provider, ok := c.dataProviders.Get(id)
	if !ok {
		c.writeBaseErrorResponse(ctx, err, "unsubscribe")
		c.logger.Debug().Err(err).Msg("no active subscription with such ID found")
		return
	}

	err = provider.Close()
	if err != nil {
		c.writeBaseErrorResponse(ctx, err, "unsubscribe")
		return
	}

	c.dataProviders.Remove(id)
	c.writeUnsubscribeOkResponse(ctx, id)
}

func (c *Controller) handleListSubscriptions(ctx context.Context) {
	var subs []*models.SubscriptionEntry

	err := c.dataProviders.ForEach(func(id uuid.UUID, provider dp.DataProvider) error {
		subs = append(subs, &models.SubscriptionEntry{
			ID:    id.String(),
			Topic: provider.Topic(),
		})
		return nil
	})

	if err != nil {
		c.writeBaseErrorResponse(ctx, err, "list_subscriptions")
		c.logger.Debug().Err(err).Msg("error listing subscriptions")
		return
	}

	resp := models.ListSubscriptionsMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			Success: true,
		},
		Subscriptions: subs,
	}
	c.writeResponse(ctx, resp)
}

func (c *Controller) shutdownConnection() {
	err := c.conn.Close()
	if err != nil {
		c.logger.Error().Err(err).Msg("error closing connection")
	}

	err = c.dataProviders.ForEach(func(_ uuid.UUID, dp dp.DataProvider) error {
		//TODO: why did i think it's a good idea to return error in Close()? it's messy now
		err = dp.Close()
		if err != nil {
			c.logger.Error().Err(err).Msg("error closing data provider")
		}

		return nil
	})

	if err != nil {
		c.logger.Error().Err(err).Msg("error closing data provider")
	}

	c.dataProviders.Clear()

	// drain the channel as some providers may still send data to it during shutdown
	go func() {
		for range c.multiplexedStream {
		}
	}()

	c.dataProvidersGroup.Wait()
	close(c.multiplexedStream)
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
				if errors.Is(err, websocket.ErrCloseSent) {
					return err
				}

				c.writeBaseErrorResponse(ctx, err, "")
				c.logger.Debug().Err(err).Msg("failed to send ping")
				return fmt.Errorf("failed to write ping message: %w", err)
			}
		}
	}
}

func (c *Controller) writeBaseErrorResponse(ctx context.Context, err error, action string) {
	request := models.BaseMessageResponse{
		Action:       action,
		Success:      false,
		ErrorMessage: err.Error(),
	}

	c.writeResponse(ctx, request)
}

func (c *Controller) writeSubscribeOkResponse(ctx context.Context, id uuid.UUID) {
	request := models.SubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			Action:  "subscribe",
			Success: true,
		},
		ID: id.String(),
	}

	c.writeResponse(ctx, request)
}

func (c *Controller) writeUnsubscribeOkResponse(ctx context.Context, id uuid.UUID) {
	request := models.UnsubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			Action:  "unsubscribe",
			Success: true,
		},
		ID: id.String(),
	}

	c.writeResponse(ctx, request)
}

func (c *Controller) writeResponse(ctx context.Context, response interface{}) {
	select {
	case <-ctx.Done():
		return
	case c.multiplexedStream <- response:
	}
}
