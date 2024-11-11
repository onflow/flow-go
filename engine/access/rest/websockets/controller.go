package websockets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

type Controller struct {
	logger               zerolog.Logger
	config               Config
	conn                 *websocket.Conn
	communicationChannel chan interface{}
	dataProviders        *concurrentmap.ConcurrentMap[uuid.UUID, dp.DataProvider]
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
		dataProviders:        concurrentmap.NewConcurrentMap[uuid.UUID, dp.DataProvider](),
		dataProvidersFactory: dp.NewDataProviderFactory(logger, streamApi, streamConfig),
	}
}

// HandleConnection manages the WebSocket connection, adding context and error handling.
func (c *Controller) HandleConnection(ctx context.Context) {
	//TODO: configure the connection with ping-pong and deadlines
	//TODO: spin up a response limit tracker routine
	go c.readMessagesFromClient(ctx)
	go c.writeMessagesToClient(ctx)
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

			baseMsg, validatedMsg, err := c.parseAndValidateMessage(msg)
			if err != nil {
				c.logger.Debug().Err(err).Msg("error parsing and validating client message")
				return
			}

			if err := c.handleAction(ctx, baseMsg.Action, validatedMsg); err != nil {
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

func (c *Controller) handleAction(ctx context.Context, action string, message interface{}) error {
	switch action {
	case "subscribe":
		c.handleSubscribe(ctx, message.(models.SubscribeMessageRequest))
	case "unsubscribe":
		c.handleUnsubscribe(ctx, message.(models.UnsubscribeMessageRequest))
	case "list_subscriptions":
		c.handleListSubscriptions(ctx, message.(models.ListSubscriptionsMessageRequest))
	default:
		return fmt.Errorf("unknown action type: %s", action)
	}
	return nil
}

func (c *Controller) handleSubscribe(ctx context.Context, msg models.SubscribeMessageRequest) {
	dp := c.dataProvidersFactory.NewDataProvider(ctx, c.communicationChannel, msg.Topic)
	c.dataProviders.Add(dp.ID(), dp)
	dp.Run(ctx)
}

func (c *Controller) handleUnsubscribe(ctx context.Context, msg models.UnsubscribeMessageRequest) {
	id, err := uuid.Parse(msg.ID)
	if err != nil {
		c.logger.Debug().Err(err).Msg("error parsing message ID")
		return
	}

	dp, ok := c.dataProviders.Get(id)
	if ok {
		dp.Close()
		c.dataProviders.Remove(id)
	}
}

func (c *Controller) handleListSubscriptions(ctx context.Context, msg models.ListSubscriptionsMessageRequest) {
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
