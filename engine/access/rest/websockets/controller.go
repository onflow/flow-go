package websockets

import (
	"context"
	"encoding/json"
	"fmt"
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
	PingPeriod = (PongWait * 9) / 10

	// PongWait specifies the maximum time to wait for a pong message from the client.
	PongWait = 10 * time.Second

	// WriteWait specifies the maximum duration allowed to write a message to the client.
	WriteWait = 10 * time.Second
)

type Controller struct {
	logger               zerolog.Logger
	config               Config
	conn                 *websocket.Conn
	communicationChannel chan interface{}
	dataProviders        *concurrentmap.Map[uuid.UUID, dp.DataProvider]
	dataProvidersFactory *dp.Factory
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
		dataProviders:        concurrentmap.New[uuid.UUID, dp.DataProvider](),
		dataProvidersFactory: dp.NewDataProviderFactory(logger, streamApi, streamConfig),
	}
}

// HandleConnection manages the WebSocket connection, adding context and error handling.
func (c *Controller) HandleConnection(ctx context.Context) {
	// configuring the connection with appropriate read/write deadlines and handlers.
	err := c.configureConnection()
	if err != nil {
		// TODO: add error handling here
		c.logger.Error().Err(err).Msg("error configuring connection")
		c.shutdownConnection()
		return
	}

	//TODO: spin up a response limit tracker routine
	go c.readMessagesFromClient(ctx)
	c.writeMessagesToClient(ctx)
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
	if err := c.conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return fmt.Errorf("failed to set the initial read deadline: %w", err)
	}
	// Establish a Pong handler which sets the handler for pong messages received from the peer.
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	return nil
}

// writeMessagesToClient reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
func (c *Controller) writeMessagesToClient(ctx context.Context) {
	//TODO: can it run forever? maybe we should cancel the ctx in the reader routine
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.communicationChannel:
			// TODO: handle 'response per second' limits

			// Specifies a timeout for the write operation. If the write
			// isn't completed within this duration, it fails with a timeout error.
			// SetWriteDeadline ensures the write operation does not block indefinitely
			// if the client is slow or unresponsive. This prevents resource exhaustion
			// and allows the server to gracefully handle timeouts for delayed writes.
			if err := c.conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				c.logger.Error().Err(err).Msg("failed to set the write deadline")
			}
			err := c.conn.WriteJSON(msg)
			if err != nil {
				c.logger.Error().Err(err).Msg("error writing to connection")
			}
		}
	}
}

// readMessagesFromClient continuously reads messages from a client WebSocket connection,
// processes each message, and handles actions based on the message type.
func (c *Controller) readMessagesFromClient(ctx context.Context) {
	defer c.shutdownConnection()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("context canceled, stopping read message loop")
			return
		default:
			msg, err := c.readMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
					return
				}
				c.logger.Warn().Err(err).Msg("error reading message from client")
				return
			}

			baseMsg, validatedMsg, err := c.parseAndValidateMessage(msg)
			if err != nil {
				c.logger.Debug().Err(err).Msg("error parsing and validating client message")
				return
			}

			if err := c.handleAction(ctx, validatedMsg); err != nil {
				c.logger.Warn().Err(err).Str("action", baseMsg.Action).Msg("error handling action")
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
}
