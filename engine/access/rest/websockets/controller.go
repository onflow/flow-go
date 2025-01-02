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

// writeMessages reads a messages from multiplexed stream and passes them on to a client WebSocket connection.
// The multiplexed stream channel is filled by data providers.
// The function tracks the last message sent and periodically checks for inactivity.
// If no messages are sent within InactivityTimeout and no active data providers exist,
// the connection will be closed.
func (c *Controller) writeMessages(ctx context.Context) error {
	inactivityTicker := time.NewTicker(c.config.InactivityTimeout / 10)
	defer inactivityTicker.Stop()

	lastMessageSentAt := time.Now()

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
				return nil
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

// readMessages continuously reads messages from a client WebSocket connection,
// validates each message, and processes it based on the message type.
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

		err := c.handleMessage(ctx, message)
		if err != nil {
			c.writeErrorResponse(
				ctx,
				err,
				wrapErrorMessage(InvalidMessage, "error parsing message", "", "", ""))
			continue
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
