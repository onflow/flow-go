package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/concurrentmap"
	"github.com/onflow/flow-go/utils/concurrentticker"
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
	limiter             *rate.Limiter
	inactivityTracker   *concurrentticker.Ticker
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
		limiter:             rate.NewLimiter(rate.Limit(config.MaxResponsesPerSecond), 1),
		inactivityTracker:   concurrentticker.NewTicker(config.InactivityTimeout),
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
		c.logger.Error().Err(err).Msg("error configuring keepalive connection")
		return
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return c.keepalive(gCtx)
	})
	g.Go(func() error {
		return c.writeMessages(gCtx)
	})
	g.Go(func() error {
		return c.readMessages(gCtx)
	})
	g.Go(func() error {
		return c.monitorInactivity(ctx)
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

// monitorInactivity periodically checks for inactivity on the connection.
//
// Expected behavior:
// - Terminates when all data providers are unsubscribed.
// - Resets based on activity such as adding/removing subscriptions.
//
// Parameters:
// - ctx: Context to control cancellation and timeouts.
func (c *Controller) monitorInactivity(ctx context.Context) error {
	defer c.inactivityTracker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.inactivityTracker.C():
			if c.dataProviders.Size() == 0 {
				// Optionally send a message to the client indicating the reason for closure.
				c.logger.Info().Msg("Connection inactive, closing due to timeout.")
				return fmt.Errorf("no recent activity for %v", c.config.InactivityTimeout)
			}
		}
	}
}

// checkInactivity checks if there are no active data providers
// and resets the inactivity timer. This function should be called after removing a data provider
// to update the inactivity tracker and ensure it reflects the current state.
func (c *Controller) checkInactivity() {
	if c.dataProviders.Size() == 0 {
		c.inactivityTracker.Reset(c.config.InactivityTimeout)
	}
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
			return nil
		case <-pingTicker.C:
			err := c.conn.WriteControl(websocket.PingMessage, time.Now().Add(WriteWait))
			if err != nil {
				if errors.Is(err, websocket.ErrCloseSent) {
					return err
				}

				return fmt.Errorf("error sending ping: %w", err)
			}
		}
	}
}

// writeMessages reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
//
// Expected errors during normal operation:
// - context.Canceled if the client disconnected
func (c *Controller) writeMessages(ctx context.Context) error {
	defer func() {
		// drain the channel as some providers may still send data to it after this routine shutdowns
		// so, in order to not run into deadlock there should be at least 1 reader on the channel
		go func() {
			for range c.multiplexedStream {
			}
		}()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-c.multiplexedStream:
			if !ok {
				return fmt.Errorf("multiplexed stream closed")
			}

			// wait for the rate limiter to allow the next message write.
			if err := c.limiter.WaitN(ctx, 1); err != nil {
				return fmt.Errorf("rate limiter wait failed: %w", err)
			}

			// Specifies a timeout for the write operation. If the write
			// isn't completed within this duration, it fails with a timeout error.
			// SetWriteDeadline ensures the write operation does not block indefinitely
			// if the client is slow or unresponsive. This prevents resource exhaustion
			// and allows the server to gracefully handle timeouts for delayed writes.
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
// validates each message, and processes it based on the message type.
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

			c.writeErrorResponse(
				ctx,
				err,
				wrapErrorMessage(InvalidMessage, "error reading message", "", "", ""))
			continue
		}

		err := c.parseAndValidateMessage(ctx, message)
		if err != nil {
			c.logger.Debug().Msgf("!!!! parseAndValidateMessage error %v", err)
			c.writeErrorResponse(
				ctx,
				err,
				wrapErrorMessage(InvalidMessage, "error parsing message", "", "", ""))
			continue
		}
		c.logger.Debug().Msgf("!!!! success")
	}
}

func (c *Controller) parseAndValidateMessage(ctx context.Context, message json.RawMessage) error {
	c.logger.Debug().Msg("!!!! parseAndValidateMessage")

	var baseMsg models.BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return fmt.Errorf("error unmarshalling base message: %w", err)
	}

	switch baseMsg.Action {
	case models.SubscribeAction:
		var subscribeMsg models.SubscribeMessageRequest
		if err := json.Unmarshal(message, &subscribeMsg); err != nil {
			return fmt.Errorf("error unmarshalling subscribe message: %w", err)
		}
		c.handleSubscribe(ctx, subscribeMsg)

	case models.UnsubscribeAction:
		var unsubscribeMsg models.UnsubscribeMessageRequest
		if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
			return fmt.Errorf("error unmarshalling unsubscribe message: %w", err)
		}
		c.handleUnsubscribe(ctx, unsubscribeMsg)

	case models.ListSubscriptionsAction:
		var listMsg models.ListSubscriptionsMessageRequest
		if err := json.Unmarshal(message, &listMsg); err != nil {
			return fmt.Errorf("error unmarshalling list subscriptions message: %w", err)
		}
		c.handleListSubscriptions(ctx, listMsg)

	default:
		c.logger.Debug().Str("action", baseMsg.Action).Msg("unknown action type")
		return fmt.Errorf("unknown action type: %s", baseMsg.Action)
	}

	return nil
}

func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	// register new provider
	provider, err := c.dataProviderFactory.NewDataProvider(ctx, msg.Topic, msg.Arguments, c.multiplexedStream)
	if err != nil {
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(InvalidArgument, "error creating data provider", msg.ClientMessageID, models.SubscribeAction, ""),
		)
		return
	}
	c.dataProviders.Add(provider.ID(), provider)

	// write OK response to client
	responseOk := models.SubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			ClientMessageID: msg.ClientMessageID,
			Success:         true,
			SubscriptionID:  provider.ID().String(),
		},
	}
	c.writeResponse(ctx, responseOk)

	// run provider
	c.dataProvidersGroup.Add(1)
	go func() {
		err = provider.Run()
		if err != nil {
			c.writeErrorResponse(
				ctx,
				err,
				wrapErrorMessage(SubscriptionError, "subscription finished with error", "", "", ""),
			)
		}

		c.dataProvidersGroup.Done()
		c.dataProviders.Remove(provider.ID())
		c.checkInactivity()
	}()
}

func (c *Controller) handleUnsubscribe(ctx context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.SubscriptionID)
	if err != nil {
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(InvalidArgument, "error parsing subscription ID", msg.ClientMessageID, models.UnsubscribeAction, msg.SubscriptionID),
		)
		return
	}

	provider, ok := c.dataProviders.Get(id)
	if !ok {
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(NotFound, "subscription not found", msg.ClientMessageID, models.UnsubscribeAction, msg.SubscriptionID),
		)
		return
	}

	provider.Close()
	c.dataProviders.Remove(id)
	c.checkInactivity()

	responseOk := models.UnsubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			ClientMessageID: msg.ClientMessageID,
			Success:         true,
			SubscriptionID:  msg.SubscriptionID,
		},
	}
	c.writeResponse(ctx, responseOk)
}

func (c *Controller) handleListSubscriptions(ctx context.Context, msg models.ListSubscriptionsMessageRequest) {
	var subs []*models.SubscriptionEntry
	err := c.dataProviders.ForEach(func(id uuid.UUID, provider dp.DataProvider) error {
		subs = append(subs, &models.SubscriptionEntry{
			ID:    id.String(),
			Topic: provider.Topic(),
		})
		return nil
	})

	if err != nil {
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(NotFound, "error listing subscriptions", msg.ClientMessageID, models.ListSubscriptionsAction, ""),
		)
		return
	}

	responseOk := models.ListSubscriptionsMessageResponse{
		Success:         true,
		ClientMessageID: msg.ClientMessageID,
		Subscriptions:   subs,
	}
	c.writeResponse(ctx, responseOk)
}

func (c *Controller) shutdownConnection() {
	err := c.conn.Close()
	if err != nil {
		c.logger.Debug().Err(err).Msg("error closing connection")
	}

	err = c.dataProviders.ForEach(func(_ uuid.UUID, provider dp.DataProvider) error {
		provider.Close()
		return nil
	})
	if err != nil {
		c.logger.Debug().Err(err).Msg("error closing data provider")
	}

	c.dataProviders.Clear()
	c.dataProvidersGroup.Wait()
	close(c.multiplexedStream)
}

func (c *Controller) writeErrorResponse(ctx context.Context, err error, msg models.BaseMessageResponse) {
	c.logger.Debug().Err(err).Msg(msg.Error.Message)
	c.writeResponse(ctx, msg)
}

func (c *Controller) writeResponse(ctx context.Context, response interface{}) {
	select {
	case <-ctx.Done():
		return
	case c.multiplexedStream <- response:
	}
}

func wrapErrorMessage(code Code, message string, msgId string, action string, subscriptionID string) models.BaseMessageResponse {
	return models.BaseMessageResponse{
		ClientMessageID: msgId,
		Success:         false,
		SubscriptionID:  subscriptionID,
		Error: models.ErrorMessage{
			Code:    int(code),
			Message: message,
			Action:  action,
		},
	}
}
