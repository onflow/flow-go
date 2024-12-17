package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/concurrentmap"
)

type Controller struct {
	logger               zerolog.Logger
	config               Config
	conn                 WebsocketConnection
	communicationChannel chan interface{}
	dataProviders        *concurrentmap.Map[uuid.UUID, dp.DataProvider]
	dataProviderFactory  dp.DataProviderFactory
	shutdownOnce         sync.Once
}

func NewWebSocketController(
	logger zerolog.Logger,
	config Config,
	dataProviderFactory dp.DataProviderFactory,
	conn WebsocketConnection,
) *Controller {
	return &Controller{
		logger:               logger.With().Str("component", "websocket-controller").Logger(),
		config:               config,
		conn:                 conn,
		communicationChannel: make(chan interface{}), //TODO: should it be buffered chan?
		dataProviders:        concurrentmap.New[uuid.UUID, dp.DataProvider](),
		dataProviderFactory:  dataProviderFactory,
		shutdownOnce:         sync.Once{},
	}
}

// HandleConnection manages the WebSocket connection, adding context and error handling.
func (c *Controller) HandleConnection(ctx context.Context) {
	//TODO: configure the connection with ping-pong and deadlines
	//TODO: spin up a response limit tracker routine
	defer c.shutdownConnection()
	go c.readMessages(ctx)
	c.writeMessages(ctx)
}

// writeMessages reads a messages from communication channel and passes them on to a client WebSocket connection.
// The communication channel is filled by data providers. Besides, the response limit tracker is involved in
// write message regulation
func (c *Controller) writeMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.communicationChannel:
			if !ok {
				return
			}
			c.logger.Debug().Msgf("read message from communication channel: %s", msg)

			// TODO: handle 'response per second' limits

			err := c.conn.WriteJSON(msg)
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) ||
					websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}

				c.logger.Error().Err(err).Msg("error writing to connection")
				return
			}

			c.logger.Debug().Msg("written message to client")
		}
	}
}

// readMessages continuously reads messages from a client WebSocket connection,
// processes each message, and handles actions based on the message type.
func (c *Controller) readMessages(ctx context.Context) {
	for {
		msg, err := c.readMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) ||
				websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}

			c.logger.Debug().Err(err).Msg("error reading message from client")
			continue
		}

		baseMsg, validatedMsg, err := c.parseAndValidateMessage(msg)
		if err != nil {
			c.logger.Debug().Err(err).Msg("error parsing and validating client message")
			//TODO: write error to error channel
			continue
		}

		if err := c.handleAction(ctx, validatedMsg); err != nil {
			c.logger.Debug().Err(err).Str("action", baseMsg.Action).Msg("error handling action")
			//TODO: write error to error channel
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
	dp, err := c.dataProviderFactory.NewDataProvider(ctx, msg.Topic, msg.Arguments, c.communicationChannel)
	if err != nil {
		// TODO: handle error here
		c.logger.Error().Err(err).Msgf("error while creating data provider for topic: %s", msg.Topic)
	}

	c.dataProviders.Add(dp.ID(), dp)

	// firstly, we want to write OK response to client and only after that we can start providing actual data
	response := models.SubscribeMessageResponse{
		BaseMessageResponse: models.BaseMessageResponse{
			Success: true,
		},
		Topic: dp.Topic(),
		ID:    dp.ID().String(),
	}

	select {
	case <-ctx.Done():
		return
	case c.communicationChannel <- response:
	}

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
		defer func() {
			if err := c.conn.Close(); err != nil {
				c.logger.Warn().Err(err).Msg("error closing connection")
			}
		}()

		c.logger.Debug().Msg("shutting down connection")

		_ = c.dataProviders.ForEach(func(id uuid.UUID, dp dp.DataProvider) error {
			err := dp.Close()
			c.logger.Error().Err(err).
				Str("data_provider", id.String()).
				Msg("error closing data provider")
			return nil
		})

		c.dataProviders.Clear()
	})
}
