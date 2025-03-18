// Package websockets provides a number of abstractions for managing WebSocket connections.
// It supports handling client subscriptions, sending messages, and maintaining
// the lifecycle of WebSocket connections with robust keepalive mechanisms.
//
// Overview
//
// The architecture of this package consists of three main components:
//
// 1. **Connection**: Responsible for providing a channel that allows the client
//    to communicate with the server. It encapsulates WebSocket-level operations
//    such as sending and receiving messages.
// 2. **Data Providers**: Standalone units responsible for fetching data from
//    the blockchain (protocol). These providers act as sources of data that are
//    sent to clients based on their subscriptions.
// 3. **Controller**: Acts as a mediator between the connection and data providers.
//    It governs client subscriptions, handles client requests and responses,
//    validates messages, and manages error handling. The controller ensures smooth
//    coordination between the client and the data-fetching units.
//
// Basically, it is an N:1:1 approach: N data providers, 1 controller, 1 websocket connection.
// This allows a client to receive messages from different subscriptions over a single connection.
//
// ### Controller Details
//
// The `Controller` is the core component that coordinates the interactions between
// the client and data providers. It achieves this through three routines that run
// in parallel (writer, reader, and keepalive routine). If any of the three routines
// fails with an error, the remaining routines will be canceled using the provided
// context to ensure proper cleanup and termination.
//
// 1. **Reader Routine**:
//    - Reads messages from the client WebSocket connection.
//    - Parses and validates the messages.
//    - Handles the messages by triggering the appropriate actions, such as subscribing
//      to a topic or unsubscribing from an existing subscription.
//    - Ensures proper validation of message formats and data before passing them to
//      the internal handlers.
//
// 2. **Writer Routine**:
//    - Listens to the `multiplexedStream`, which is a channel filled by data providers
//      with messages that clients have subscribed to.
//    - Writes these messages to the client WebSocket connection.
//    - Ensures the outgoing messages respect the required deadlines to maintain the
//      stability of the connection.
//
// 3. **Keepalive Routine**:
//    - Periodically sends a WebSocket ping control message to the client to indicate
//      that the controller and all its subscriptions are working as expected.
//    - Ensures the connection remains clean and avoids timeout scenarios due to
//      inactivity.
//    - Resets the connection's read deadline whenever a pong message is received.
//
// Example
//
// Usage typically involves creating a `Controller` instance and invoking its
// `HandleConnection` method to manage a single WebSocket connection:
//
//     logger := zerolog.New(os.Stdout)
//     config := websockets.Config{/* configuration options */}
//     conn := /* a WebsocketConnection implementation */
//     factory := /* a DataProviderFactory implementation */
//
//     controller := websockets.NewWebSocketController(logger, config, conn, factory)
//     ctx := context.Background()
//     controller.HandleConnection(ctx)
//
//
// Package Constants
//
// This package expects constants like `PongWait` and `WriteWait` for controlling
// the read/write deadlines. They need to be defined in your application as appropriate.

package websockets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

// ErrMaxSubscriptionsReached is returned when the maximum number of active subscriptions per connection is exceeded.
var ErrMaxSubscriptionsReached = errors.New("maximum number of subscriptions reached")

type Controller struct {
	logger zerolog.Logger
	config Config
	conn   WebsocketConnection

	// The `multiplexedStream` is a core channel used for communication between the
	// `Controller` and Data Providers. Its lifecycle is as follows:
	//
	// 1. **Data Providers**:
	//    - Data providers write their data into this channel, which is consumed by
	//      the writer routine to send messages to the client.
	// 2. **Reader Routine**:
	//    - Writes OK/error responses to the channel as a result of processing client messages.
	// 3. **Writer Routine**:
	//    - Reads messages from this channel and forwards them to the client WebSocket connection.
	//
	// 4. **Channel Closing**:
	//      The intention to close the channel comes from the reader-from-this-channel routines (controller's routines),
	//      not the writer-to-this-channel routines (data providers).
	//      Therefore, we have to signal the data providers to stop writing, wait for them to finish write operations,
	//      and only after that we can close the channel.
	//
	//    - The `Controller` is responsible for starting and managing the lifecycle of the channel.
	//    - If an unrecoverable error occurs in any of the three routines (reader, writer, or keepalive),
	//      the parent context is canceled. This triggers data providers to stop their work.
	//    - The `multiplexedStream` will not be closed until all data providers signal that
	//      they have stopped writing to it via the `dataProvidersGroup` wait group.
	//
	// 5. **Edge Case - Writer Routine Finished Before Providers**:
	//    - If the writer routine finishes before all data providers, a separate draining routine
	//      ensures that the `multiplexedStream` is fully drained to prevent deadlocks.
	//      All remaining messages in this case will be discarded.
	//
	// This design ensures that the channel is only closed when it is safe to do so, avoiding
	// issues such as sending on a closed channel while maintaining proper cleanup.
	multiplexedStream chan interface{}

	dataProviders       *concurrentmap.Map[SubscriptionID, dp.DataProvider]
	dataProviderFactory dp.DataProviderFactory
	dataProvidersGroup  *sync.WaitGroup
	limiter             *rate.Limiter
}

func NewWebSocketController(
	logger zerolog.Logger,
	config Config,
	conn WebsocketConnection,
	dataProviderFactory dp.DataProviderFactory,
) *Controller {
	var limiter *rate.Limiter
	if config.MaxResponsesPerSecond > 0 {
		limiter = rate.NewLimiter(rate.Limit(config.MaxResponsesPerSecond), 1)
	}

	return &Controller{
		logger:              logger.With().Str("component", "websocket-controller").Logger(),
		config:              config,
		conn:                conn,
		multiplexedStream:   make(chan interface{}),
		dataProviders:       concurrentmap.New[SubscriptionID, dp.DataProvider](),
		dataProviderFactory: dataProviderFactory,
		dataProvidersGroup:  &sync.WaitGroup{},
		limiter:             limiter,
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

// keepalive sends a ping message periodically to keep the WebSocket connection alive
// and avoid timeouts.
func (c *Controller) keepalive(ctx context.Context) error {
	defer func() {
		// gracefully handle panics from github.com/gorilla/websocket
		if r := recover(); r != nil {
			c.logger.Warn().Interface("recovered_context", r).Msg("keepalive routine recovered from panic")
		}
	}()

	pingTicker := time.NewTicker(PingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-pingTicker.C:
			err := c.conn.WriteControl(websocket.PingMessage, time.Now().Add(WriteWait))
			if err != nil {
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					return err
				}

				return fmt.Errorf("error sending ping: %w", err)
			}
		}
	}
}

// writeMessages reads a messages from multiplexed stream and passes them on to a client WebSocket connection.
// The multiplexed stream channel is filled by data providers.
// The function tracks the last message sent and periodically checks for inactivity.
// If no messages are sent within InactivityTimeout and no active data providers exist,
// the connection will be closed.
func (c *Controller) writeMessages(ctx context.Context) error {
	defer func() {
		// gracefully handle panics from github.com/gorilla/websocket
		if r := recover(); r != nil {
			c.logger.Warn().Interface("recovered_context", r).Msg("writer routine recovered from panic")
		}
	}()

	defer func() {
		// drain the channel as some providers may still send data to it after this routine shutdowns
		// so, in order to not run into deadlock there should be at least 1 reader on the channel
		go func() {
			for range c.multiplexedStream {
			}
		}()
	}()

	inactivityTicker := time.NewTicker(c.inactivityTickerPeriod())
	defer inactivityTicker.Stop()

	lastMessageSentAt := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-c.multiplexedStream:
			if !ok {
				return nil
			}

			if err := c.checkRateLimit(ctx); err != nil {
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

			lastMessageSentAt = time.Now()

		case <-inactivityTicker.C:
			hasNoActiveSubscriptions := c.dataProviders.Size() == 0
			exceedsInactivityTimeout := time.Since(lastMessageSentAt) > c.config.InactivityTimeout
			if hasNoActiveSubscriptions && exceedsInactivityTimeout {
				c.logger.Debug().
					Dur("timeout", c.config.InactivityTimeout).
					Msg("connection inactive, closing due to timeout")
				return fmt.Errorf("no recent activity for %v", c.config.InactivityTimeout)
			}
		}
	}
}

// inactivityTickerPeriod determines the interval at which the inactivity ticker is triggered.
//
// The inactivity ticker is used in the `writeMessages` routine to monitor periods of inactivity
// in outgoing messages. If no messages are sent within the defined inactivity timeout
// and there are no active data providers, the WebSocket connection will be terminated.
func (c *Controller) inactivityTickerPeriod() time.Duration {
	return c.config.InactivityTimeout / 10
}

// readMessages continuously reads messages from a client WebSocket connection,
// validates each message, and processes it based on the message type.
func (c *Controller) readMessages(ctx context.Context) error {
	defer func() {
		// gracefully handle panics from github.com/gorilla/websocket
		if r := recover(); r != nil {
			c.logger.Warn().Interface("recovered_context", r).Msg("reader routine recovered from panic")
		}
	}()

	for {
		select {
		// ctx.Done() is necessary in readMessages() to gracefully handle the termination of the connection
		// and prevent a potential panic ("repeated read on failed websocket connection"). If an error occurs in writeMessages(),
		// it indirectly affect the keepalive mechanism.
		// This can stop periodic ping messages from being sent to the server, causing the server to close the connection.
		// Without ctx.Done(), readMessages could continue blocking on a read operation, eventually encountering an i/o timeout
		// when no data arrives. By monitoring ctx.Done(), we ensure that readMessages exits promptly when the context is canceled
		// due to errors elsewhere in the system or intentional shutdown.
		case <-ctx.Done():
			return nil
		default:
			var message json.RawMessage
			if err := c.conn.ReadJSON(&message); err != nil {
				var closeErr *websocket.CloseError
				if errors.As(err, &closeErr) {
					return err
				}

				err = fmt.Errorf("error reading message: %w", err)
				c.writeErrorResponse(
					ctx,
					err,
					wrapErrorMessage(http.StatusBadRequest, err.Error(), "", ""),
				)
				continue
			}

			err := c.handleMessage(ctx, message)
			if err != nil {
				err = fmt.Errorf("error parsing message: %w", err)
				c.writeErrorResponse(
					ctx,
					err,
					wrapErrorMessage(http.StatusBadRequest, err.Error(), "", ""),
				)
				continue
			}
		}
	}

}

func (c *Controller) handleMessage(ctx context.Context, message json.RawMessage) error {
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

// handleSubscribe processes a subscription request.
//
// Expected error returns during normal operations:
//   - ErrMaxSubscriptionsReached: if the maximum number of active subscriptions per connection is exceeded.
func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	// Check if the maximum number of active subscriptions per connection has been reached.
	// If the limit is exceeded, an error is returned, and the subscription request is rejected.
	if uint64(c.dataProviders.Size()) >= c.config.MaxSubscriptionsPerConnection {
		err := fmt.Errorf("error creating new subscription: %w", ErrMaxSubscriptionsReached)
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(http.StatusServiceUnavailable, err.Error(), models.SubscribeAction, msg.SubscriptionID),
		)
		return
	}

	subscriptionID, err := c.parseOrCreateSubscriptionID(msg.SubscriptionID)
	if err != nil {
		err = fmt.Errorf("error parsing subscription id: %w", err)
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(http.StatusBadRequest, err.Error(), models.SubscribeAction, msg.SubscriptionID),
		)
		return
	}

	// register new provider
	provider, err := c.dataProviderFactory.NewDataProvider(ctx, subscriptionID.String(), msg.Topic, msg.Arguments, c.multiplexedStream)
	if err != nil {
		err = fmt.Errorf("error creating data provider: %w", err)
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(http.StatusBadRequest, err.Error(), models.SubscribeAction, subscriptionID.String()),
		)
		return
	}
	c.dataProviders.Add(subscriptionID, provider)

	// write OK response to client
	responseOk := models.SubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			SubscriptionID: subscriptionID.String(),
			Action:         models.SubscribeAction,
		},
	}
	c.writeResponse(ctx, responseOk)

	// run provider
	c.dataProvidersGroup.Add(1)
	go func() {
		err = provider.Run()
		// return the error to the client for all errors except context.Canceled.
		// context.Canceled is returned during graceful shutdown of a subscription
		if err != nil && !errors.Is(err, context.Canceled) {
			err = fmt.Errorf("internal error: %w", err)
			c.writeErrorResponse(
				ctx,
				err,
				wrapErrorMessage(http.StatusInternalServerError, err.Error(),
					models.SubscribeAction, subscriptionID.String()),
			)
		}

		c.dataProvidersGroup.Done()
		c.dataProviders.Remove(subscriptionID)
	}()
}

func (c *Controller) handleUnsubscribe(ctx context.Context, msg models.UnsubscribeMessageRequest) {
	subscriptionID, err := ParseClientSubscriptionID(msg.SubscriptionID)
	if err != nil {
		err = fmt.Errorf("error parsing subscription id: %w", err)
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(http.StatusBadRequest, err.Error(), models.UnsubscribeAction, msg.SubscriptionID),
		)
		return
	}

	provider, ok := c.dataProviders.Get(subscriptionID)
	if !ok {
		c.writeErrorResponse(
			ctx,
			err,
			wrapErrorMessage(http.StatusNotFound, "subscription not found",
				models.UnsubscribeAction, subscriptionID.String()),
		)
		return
	}

	provider.Close()
	c.dataProviders.Remove(subscriptionID)

	responseOk := models.UnsubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			SubscriptionID: subscriptionID.String(),
			Action:         models.UnsubscribeAction,
		},
	}
	c.writeResponse(ctx, responseOk)
}

func (c *Controller) handleListSubscriptions(ctx context.Context, _ models.ListSubscriptionsMessageRequest) {
	var subs []*models.SubscriptionEntry
	_ = c.dataProviders.ForEach(func(id SubscriptionID, provider dp.DataProvider) error {
		subs = append(subs, &models.SubscriptionEntry{
			SubscriptionID: id.String(),
			Topic:          provider.Topic(),
			Arguments:      provider.Arguments(),
		})
		return nil
	})

	responseOk := models.ListSubscriptionsMessageResponse{
		Subscriptions: subs,
		Action:        models.ListSubscriptionsAction,
	}
	c.writeResponse(ctx, responseOk)
}

func (c *Controller) shutdownConnection() {
	err := c.conn.Close()
	if err != nil {
		c.logger.Debug().Err(err).Msg("error closing connection")
	}

	_ = c.dataProviders.ForEach(func(_ SubscriptionID, provider dp.DataProvider) error {
		provider.Close()
		return nil
	})

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

func wrapErrorMessage(code int, message string, action string, subscriptionID string) models.BaseMessageResponse {
	return models.BaseMessageResponse{
		SubscriptionID: subscriptionID,
		Error: models.ErrorMessage{
			Code:    code,
			Message: message,
		},
		Action: action,
	}
}

func (c *Controller) parseOrCreateSubscriptionID(id string) (SubscriptionID, error) {
	newId, err := NewSubscriptionID(id)
	if err != nil {
		return SubscriptionID{}, err
	}

	if c.dataProviders.Has(newId) {
		return SubscriptionID{}, fmt.Errorf("subscription ID is already in use: %s", newId)
	}

	return newId, nil
}

// checkRateLimit checks the controller rate limit and blocks until there is room to send a response.
// An error is returned if the context is canceled or the expected wait time exceeds the context's
// deadline.
func (c *Controller) checkRateLimit(ctx context.Context) error {
	if c.limiter == nil {
		return nil
	}

	return c.limiter.WaitN(ctx, 1)
}
