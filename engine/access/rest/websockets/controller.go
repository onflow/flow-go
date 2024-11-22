package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

const (
	// PingPeriod defines the interval at which ping messages are sent to the client.
	// This value must be less than pongWait.
	PingPeriod = (pongWait * 9) / 10

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

type Controller struct {
	logger               zerolog.Logger
	config               Config
	conn                 *websocket.Conn
	communicationChannel chan interface{}
	errorChannel         chan error
	dataProviders        *concurrentmap.Map[uuid.UUID, dp.DataProvider]
	dataProvidersFactory *dp.Factory

	shutdownOnce sync.Once // Ensures shutdown is only called once
	shutdown     bool
}

func NewWebSocketController(
	logger zerolog.Logger,
	config Config,
	streamApi state_stream.API,
	streamConfig backend.Config,
	conn *websocket.Conn,
) *Controller {
	return &Controller{
		logger:               logger.With().Str("component", "websocket-controller").Logger(),
		config:               config,
		conn:                 conn,
		communicationChannel: make(chan interface{}), //TODO: should it be buffered chan?
		errorChannel:         make(chan error, 1),
		dataProviders:        concurrentmap.New[uuid.UUID, dp.DataProvider](),
		dataProvidersFactory: dp.NewDataProviderFactory(logger, streamApi, streamConfig),
	}
}

// HandleConnection manages the lifecycle of a WebSocket connection,
// including setup, message processing, and graceful shutdown.
//
// Parameters:
// - ctx: The context for controlling cancellation and timeouts.
func (c *Controller) HandleConnection(ctx context.Context) {
	defer close(c.errorChannel)
	// configuring the connection with appropriate read/write deadlines and handlers.
	err := c.configureConnection()
	if err != nil {
		// TODO: add error handling here
		c.logger.Error().Err(err).Msg("error configuring connection")
		c.shutdownConnection()
		return
	}

	//TODO: spin up a response limit tracker routine

	// for track all goroutines and error handling
	var wg sync.WaitGroup

	c.startProcess(&wg, ctx, c.readMessagesFromClient)
	c.startProcess(&wg, ctx, c.keepalive)
	c.startProcess(&wg, ctx, c.writeMessagesToClient)

	select {
	case err := <-c.errorChannel:
		c.logger.Error().Err(err).Msg("error detected in one of the goroutines")
		//TODO: add error handling here
		c.shutdownConnection()
	case <-ctx.Done():
		// Context canceled, shut down gracefully
		c.shutdownConnection()
	}

	// Wait for all goroutines
	wg.Wait()
}

// startProcess is a helper function to start a goroutine for a given process
// and ensure it is tracked via a sync.WaitGroup.
//
// Parameters:
// - wg: The wait group to track goroutines.
// - ctx: The context for cancellation.
// - process: The function to run in a new goroutine.
//
// No errors are expected during normal operation.
func (c *Controller) startProcess(wg *sync.WaitGroup, ctx context.Context, process func(context.Context) error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		err := process(ctx)
		if err != nil {
			// Check if shutdown has already been called, to avoid multiple shutdowns
			if c.shutdown {
				c.logger.Warn().Err(err).Msg("error detected after shutdown initiated, ignoring")
				return
			}

			c.errorChannel <- err
		}
	}()
}

// configureConnection sets up the WebSocket connection with a read deadline
// and a handler for receiving pong messages from the client.
//
// The function does the following:
//  1. Sets an initial read deadline to ensure the server doesn't wait indefinitely
//     for a pong message from the client. If no message is received within the
//     specified `pongWait` duration, the connection will be closed.
//  2. Establishes a Pong handler that resets the read deadline every time a pong
//     message is received from the client, allowing the server to continue waiting
//     for further pong messages within the new deadline.
func (c *Controller) configureConnection() error {
	// Set the initial read deadline for the first pong message
	// The Pong handler itself only resets the read deadline after receiving a Pong.
	// It doesn't set an initial deadline. The initial read deadline is crucial to prevent the server from waiting
	// forever if the client doesn't send Pongs.
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return fmt.Errorf("failed to set the initial read deadline: %w", err)
	}
	// Establish a Pong handler which sets the handler for pong messages received from the peer.
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	return nil
}

// writeMessagesToClient reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
//
// No errors are expected during normal operation.
func (c *Controller) writeMessagesToClient(ctx context.Context) error {
	//TODO: can it run forever? maybe we should cancel the ctx in the reader routine
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-c.communicationChannel:
			// TODO: handle 'response per second' limits

			// Specifies a timeout for the write operation. If the write
			// isn't completed within this duration, it fails with a timeout error.
			// SetWriteDeadline ensures the write operation does not block indefinitely
			// if the client is slow or unresponsive. This prevents resource exhaustion
			// and allows the server to gracefully handle timeouts for delayed writes.
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				c.logger.Error().Err(err).Msg("failed to set the write deadline")
				return err
			}
			err := c.conn.WriteJSON(msg)
			if err != nil {
				c.logger.Error().Err(err).Msg("error writing to connection")
				return err
			}
		}
	}
}

// readMessagesFromClient continuously reads messages from a client WebSocket connection,
// processes each message, and handles actions based on the message type.
//
// No errors are expected during normal operation.
func (c *Controller) readMessagesFromClient(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("context canceled, stopping read message loop")
			return nil
		default:
			msg, err := c.readMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
					return nil
				}
				c.logger.Warn().Err(err).Msg("error reading message from client")
				return err
			}

			baseMsg, validatedMsg, err := c.parseAndValidateMessage(msg)
			if err != nil {
				c.logger.Debug().Err(err).Msg("error parsing and validating client message")
				return err
			}

			if err := c.handleAction(ctx, validatedMsg); err != nil {
				c.logger.Warn().Err(err).Str("action", baseMsg.Action).Msg("error handling action")
				return err
			}
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

func (c *Controller) parseAndValidateMessage(message json.RawMessage) (models.BaseMessageRequest, interface{}, error) {
	var baseMsg models.BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return models.BaseMessageRequest{}, nil, fmt.Errorf("error unmarshalling base message: %w", err)
	}

	var validatedMsg interface{}
	switch baseMsg.Action {
	case "subscribe":
		var subscribeMsg models.SubscribeMessageRequest
		if err := json.Unmarshal(message, &subscribeMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling subscribe message: %w", err)
		}
		//TODO: add validation logic for `topic` field
		validatedMsg = subscribeMsg

	case "unsubscribe":
		var unsubscribeMsg models.UnsubscribeMessageRequest
		if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling unsubscribe message: %w", err)
		}
		validatedMsg = unsubscribeMsg

	case "list_subscriptions":
		var listMsg models.ListSubscriptionsMessageRequest
		if err := json.Unmarshal(message, &listMsg); err != nil {
			return baseMsg, nil, fmt.Errorf("error unmarshalling list subscriptions message: %w", err)
		}
		validatedMsg = listMsg

	default:
		c.logger.Debug().Str("action", baseMsg.Action).Msg("unknown action type")
		return baseMsg, nil, fmt.Errorf("unknown action type: %s", baseMsg.Action)
	}

	return baseMsg, validatedMsg, nil
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
	dp := c.dataProvidersFactory.NewDataProvider(c.communicationChannel, msg.Topic)
	c.dataProviders.Add(dp.ID(), dp)
	dp.Run(ctx)

	//TODO: return OK response to client
	c.communicationChannel <- msg
}

func (c *Controller) handleUnsubscribe(_ context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.logger.Debug().Err(err).Msg("error parsing message ID")
		//TODO: return an error response to client
		c.communicationChannel <- err
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
	c.shutdownOnce.Do(func() {
		c.shutdown = true

		defer close(c.communicationChannel)
		defer func(conn *websocket.Conn) {
			if err := c.conn.Close(); err != nil {
				c.logger.Error().Err(err).Msg("error closing connection")
			}
		}(c.conn)

		err := c.dataProviders.ForEach(func(_ uuid.UUID, dp dp.DataProvider) error {
			dp.Close()
			return nil
		})
		if err != nil {
			c.logger.Error().Err(err).Msg("error closing data provider")
		}

		c.dataProviders.Clear()
	})
}

// keepalive sends a ping message periodically to keep the WebSocket connection alive
// and avoid timeouts.
//
// No errors are expected during normal operation.
func (c *Controller) keepalive(ctx context.Context) error {
	pingTicker := time.NewTicker(PingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			//	return ctx.Err()
			return nil
		case <-pingTicker.C:
			if err := c.sendPing(); err != nil {
				// Log error and exit the loop on failure
				c.logger.Error().Err(err).Msg("failed to send ping")
				return err
			}
		}
	}
}

// sendPing sends a periodic ping message to the WebSocket client to keep the connection alive.
func (c *Controller) sendPing() error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return fmt.Errorf("failed to set the write deadline for ping: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		return fmt.Errorf("failed to write ping message: %w", err)
	}

	return nil
}
