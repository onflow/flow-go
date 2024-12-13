package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/mock"
	connmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type WsControllerSuite struct {
	suite.Suite

	logger       zerolog.Logger
	wsConfig     Config
	streamApi    *streammock.API
	streamConfig backend.Config
}

func (s *WsControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.wsConfig = NewDefaultWebsocketConfig()
	s.streamApi = streammock.NewAPI(s.T())
	s.streamConfig = backend.Config{}
}

func TestWsControllerSuite(t *testing.T) {
	suite.Run(t, new(WsControllerSuite))
}

// TestSubscribeRequest tests the subscribe to topic flow.
// We emulate a request message from a client, and a response message from a controller.
func (s *WsControllerSuite) TestSubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{}, 1)

		dataProvider.On("ID").Return(id)
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				for range done {
				}
			}).
			Return(nil).
			Once()

		request := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				MessageID: uuid.New().String(),
				Action:    models.SubscribeAction,
			},
			Topic:     "blocks",
			Arguments: nil,
		}
		requestJson, err := json.Marshal(request)
		require.NoError(t, err)

		// Simulate receiving the subscription request from the client
		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = requestJson
			}).
			Return(nil).
			Once()

		s.expectCloseConnection(conn, done)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.SubscribeMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				require.Equal(t, request.MessageID, response.MessageID)
				require.Equal(t, id.String(), response.ID)

				return websocket.ErrCloseSent
			})

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})

	s.T().Run("Parse and validate error", func(t *testing.T) {
		conn, dataProviderFactory, _ := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		type Request struct {
			Action string `json:"action"`
		}

		subscribeRequest := Request{
			Action: "SubscribeBlocks",
		}
		subscribeRequestJson, err := json.Marshal(subscribeRequest)
		require.NoError(t, err)

		// Simulate receiving the subscription request from the client
		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = subscribeRequestJson
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		s.expectCloseConnection(conn, done)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)
				require.Equal(t, int(InvalidMessage), response.Error.Code)

				return websocket.ErrCloseSent
			})

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})

	s.T().Run("Error creating data provider", func(t *testing.T) {
		conn, dataProviderFactory, _ := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("error creating data provider")).
			Once()

		done := make(chan struct{}, 1)
		s.expectSubscribeRequest(conn)
		s.expectCloseConnection(conn, done)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)
				require.Equal(t, int(InvalidArgument), response.Error.Code)

				return websocket.ErrCloseSent
			})

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})

	s.T().Run("Run error", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProvider.On("ID").Return(uuid.New())
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {}).
			Return(fmt.Errorf("error running data provider")).
			Once()

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		done := make(chan struct{}, 1)
		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)
		s.expectCloseConnection(conn, done)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)
				require.Equal(t, int(RunError), response.Error.Code)

				return websocket.ErrCloseSent
			})

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})
}

func (s *WsControllerSuite) TestUnsubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{}, 1)

		dataProvider.On("ID").Return(id)
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				for range done {
				}
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				MessageID: uuid.New().String(),
				Action:    models.UnsubscribeAction,
			},
			SubscriptionID: id.String(),
		}
		requestJson, err := json.Marshal(request)
		require.NoError(s.T(), err)

		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = requestJson
			}).
			Return(nil).
			Once()

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.UnsubscribeMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				require.Empty(t, response.Error)
				require.Equal(t, request.MessageID, response.MessageID)
				require.Equal(t, request.SubscriptionID, response.SubscriptionID)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})

	s.T().Run("Invalid subscription uuid", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{}, 1)

		dataProvider.On("ID").Return(id)
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				for range done {
				}
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				MessageID: uuid.New().String(),
				Action:    models.UnsubscribeAction,
			},
			SubscriptionID: "invalid-uuid",
		}
		requestJson, err := json.Marshal(request)
		require.NoError(s.T(), err)

		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = requestJson
			}).
			Return(nil).
			Once()

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)
				require.Equal(t, request.MessageID, response.MessageID)
				require.Equal(t, int(InvalidArgument), response.Error.Code)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})

	s.T().Run("Unsubscribe from unknown subscription", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{}, 1)

		dataProvider.On("ID").Return(id)
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				for range done {
				}
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				MessageID: uuid.New().String(),
				Action:    models.UnsubscribeAction,
			},
			SubscriptionID: uuid.New().String(),
		}
		requestJson, err := json.Marshal(request)
		require.NoError(s.T(), err)

		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = requestJson
			}).
			Return(nil).
			Once()

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)

				require.Equal(t, request.MessageID, response.MessageID)
				require.Equal(t, int(NotFound), response.Error.Code)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})
}

func (s *WsControllerSuite) TestListSubscriptions() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		done := make(chan struct{}, 1)

		id := uuid.New()
		topic := "blocks"
		dataProvider.On("ID").Return(id)
		dataProvider.On("Topic").Return(topic)
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				for range done {
				}
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)

		request := models.ListSubscriptionsMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				MessageID: uuid.New().String(),
				Action:    models.ListSubscriptionsAction,
			},
		}
		requestJson, err := json.Marshal(request)
		require.NoError(s.T(), err)

		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				msg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				*msg = requestJson
			}).
			Return(nil).
			Once()

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.ListSubscriptionsMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				require.Empty(t, response.Error)
				require.Equal(t, request.MessageID, response.MessageID)
				require.Equal(t, 1, len(response.Subscriptions))
				require.Equal(t, id.String(), response.Subscriptions[0].ID)
				require.Equal(t, topic, response.Subscriptions[0].Topic)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)
	})
}

// TestSubscribeBlocks tests the functionality for streaming blocks to a subscriber.
func (s *WsControllerSuite) TestSubscribeBlocks() {
	s.T().Run("Stream one block", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		dataProvider.On("ID").Return(id)
		// data provider might finish by its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()

		// Simulate data provider write a block to the controller
		expectedBlock := unittest.BlockFixture()
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				controller.multiplexedStream <- expectedBlock
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)
		s.expectCloseConnection(conn, done)

		// Expect a valid block to be passed to WriteJSON.
		// If we got to this point, the controller executed all its logic properly
		var actualBlock flow.Block
		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				block, ok := msg.(flow.Block)
				require.True(t, ok)
				actualBlock = block

				return websocket.ErrCloseSent
			})

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)

		require.Equal(t, expectedBlock, actualBlock)
	})

	s.T().Run("Stream many blocks", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		dataProvider.On("ID").Return(id)
		dataProvider.On("Close").Return(nil).Maybe()

		// Simulate data provider writes some blocks to the controller
		expectedBlocks := unittest.BlockFixtures(100)
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				for _, block := range expectedBlocks {
					controller.multiplexedStream <- *block
				}
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		msgID := s.expectSubscribeRequest(conn)
		s.expectSubscribeResponse(conn, msgID)
		s.expectCloseConnection(conn, done)

		i := 0
		actualBlocks := make([]*flow.Block, len(expectedBlocks))

		// Expect valid blocks to be passed to WriteJSON.
		// If we got to this point, the controller executed all its logic properly
		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				block, ok := msg.(flow.Block)
				require.True(t, ok)

				actualBlocks[i] = &block
				i += 1

				if i == len(expectedBlocks) {
					close(done)
					return websocket.ErrCloseSent
				}

				return nil
			}).
			Times(len(expectedBlocks))

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
		defer cancel()
		controller.HandleConnection(ctx)

		require.Equal(t, expectedBlocks, actualBlocks)
	})
}

// newControllerMocks initializes mock WebSocket connection, data provider, and data provider factory.
// The mocked functions are expected to be called in a case when a test is expected to reach WriteJSON function.
func newControllerMocks(t *testing.T) (*connmock.WebsocketConnection, *dpmock.DataProviderFactory, *dpmock.DataProvider) {
	conn := connmock.NewWebsocketConnection(t)
	conn.On("Close").Return(nil)
	conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()
	conn.On("SetReadDeadline", mock.Anything).Return(nil)
	conn.On("SetWriteDeadline", mock.Anything).Return(nil)

	dataProvider := dpmock.NewDataProvider(t)
	factory := dpmock.NewDataProviderFactory(t)

	return conn, factory, dataProvider
}

// expectSubscribeRequest mocks the client's subscription request.
func (s *WsControllerSuite) expectSubscribeRequest(conn *connmock.WebsocketConnection) string {
	request := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			MessageID: uuid.New().String(),
			Action:    models.SubscribeAction,
		},
		Topic: "blocks",
	}
	requestJson, err := json.Marshal(request)
	require.NoError(s.T(), err)

	// The very first message from a client is a request to subscribe to some topic
	conn.
		On("ReadJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			msg, ok := args.Get(0).(*json.RawMessage)
			require.True(s.T(), ok)
			*msg = requestJson
		}).
		Return(nil).
		Once()

	return request.MessageID
}

func (s *WsControllerSuite) expectCloseConnection(conn *connmock.WebsocketConnection, done <-chan struct{}) {
	// In the default case, no further communication is expected from the client.
	// We wait for the writer routine to signal completion, allowing us to close the connection gracefully
	conn.
		On("ReadJSON", mock.Anything).
		Return(func(msg interface{}) error {
			for range done {
			}
			return websocket.ErrCloseSent
		}).
		Once()
}

// expectSubscribeResponse mocks the subscription response sent to the client.
func (s *WsControllerSuite) expectSubscribeResponse(conn *connmock.WebsocketConnection, msgId string) {
	conn.
		On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			response, ok := args.Get(0).(models.SubscribeMessageResponse)
			require.True(s.T(), ok)
			require.Equal(s.T(), msgId, response.MessageID)
			require.Equal(s.T(), true, response.Success)
		}).
		Return(nil).
		Once()
}

func (s *WsControllerSuite) expectKeepaliveClose(conn *connmock.WebsocketConnection, done <-chan struct{}) {
	// first ping will be sent in 9 seconds, so there's no point mocking it
	conn.
		On("WriteControl", websocket.PingMessage, mock.Anything).
		Return(func(int, time.Time) error {
			for range done {
			}
			return websocket.ErrCloseSent
		}).
		Maybe()
}
