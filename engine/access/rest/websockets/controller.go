package websockets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
)

type Controller struct {
	ctx                  context.Context
	logger               zerolog.Logger
	config               *Config
	conn                 *websocket.Conn
	communicationChannel chan interface{}
	dataProviders        *ThreadSafeMap[uuid.UUID, dp.DataProvider]
	dataProvidersFactory *dp.Factory
}

func NewWebSocketController(
	ctx context.Context,
	logger zerolog.Logger,
	config *Config,
	streamApi state_stream.API,
	streamConfig backend.Config,
	conn *websocket.Conn,
) *Controller {
	return &Controller{
		ctx:                  ctx,
		logger:               logger.With().Str("component", "websocket-controller").Logger(),
		config:               config,
		conn:                 conn,
		communicationChannel: make(chan interface{}), //TODO: should it be buffered chan?
		dataProviders:        NewThreadSafeMap[uuid.UUID, dp.DataProvider](),
		dataProvidersFactory: dp.NewDataProviderFactory(logger, streamApi, streamConfig),
	}
}

// HandleConnection manages the WebSocket connection, adding context and error handling.
func (c *Controller) HandleConnection() {
	//TODO: configure the connection with ping-pong and deadlines

	go c.readMessagesFromClient(c.ctx)
	go c.writeMessagesToClient(c.ctx)
}

func (c *Controller) writeMessagesToClient(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.communicationChannel:
			// TODO: handle 'response per second' limits

			err := c.conn.WriteJSON(msg)
			if err != nil {
				c.logger.Error().Err(err).Msg("error writing to connection")
			}
		}
	}
}

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

			baseMsg, err := c.parseMessage(msg)
			if err != nil {
				c.logger.Warn().Err(err).Msg("error parsing base message")
				return
			}

			if err := c.dispatchAction(baseMsg.Action, msg); err != nil {
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

func (c *Controller) parseMessage(message json.RawMessage) (BaseMessageRequest, error) {
	var baseMsg BaseMessageRequest
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return BaseMessageRequest{}, fmt.Errorf("error unmarshalling base message: %w", err)
	}
	return baseMsg, nil
}

// dispatchAction routes the action to the appropriate handler based on the action type.
func (c *Controller) dispatchAction(action string, message json.RawMessage) error {
	switch action {
	case "subscribe":
		var subscribeMsg SubscribeMessageRequest
		if err := json.Unmarshal(message, &subscribeMsg); err != nil {
			return fmt.Errorf("error unmarshalling subscribe message: %w", err)
		}
		c.handleSubscribe(subscribeMsg)

	case "unsubscribe":
		var unsubscribeMsg UnsubscribeMessageRequest
		if err := json.Unmarshal(message, &unsubscribeMsg); err != nil {
			return fmt.Errorf("error unmarshalling unsubscribe message: %w", err)
		}
		c.handleUnsubscribe(unsubscribeMsg)

	case "list_subscriptions":
		var listMsg ListSubscriptionsMessageRequest
		if err := json.Unmarshal(message, &listMsg); err != nil {
			return fmt.Errorf("error unmarshalling list subscriptions message: %w", err)
		}
		c.handleListSubscriptions(listMsg)

	default:
		c.logger.Warn().Str("action", action).Msg("unknown action type")
		return fmt.Errorf("unknown action type: %s", action)
	}
	return nil
}

func (c *Controller) handleSubscribe(msg SubscribeMessageRequest) {
	dp := c.dataProvidersFactory.NewDataProvider(c.ctx, c.communicationChannel, msg.Topic)
	c.dataProviders.Insert(dp.ID(), dp)
	dp.Run()
}

func (c *Controller) handleUnsubscribe(msg UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.logger.Warn().Err(err).Str("topic", msg.Topic).Msg("error parsing message ID")
		return
	}

	dp, ok := c.dataProviders.Get(id)
	if ok {
		dp.Close()
		c.dataProviders.Remove(id)
	}
}

func (c *Controller) handleListSubscriptions(msg ListSubscriptionsMessageRequest) {}

func (c *Controller) shutdownConnection() {
	defer close(c.communicationChannel)
	defer func(conn *websocket.Conn) {
		if err := c.conn.Close(); err != nil {
			c.logger.Error().Err(err).Msg("error closing connection")
		}
	}(c.conn)

	c.dataProviders.ForEach(func(_ uuid.UUID, dp dp.DataProvider) {
		dp.Close()
	})
	c.dataProviders.Clear()
}
