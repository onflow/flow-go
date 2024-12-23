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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/mock"
	connmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// WsControllerSuite is a test suite for the WebSocket Controller.
type WsControllerSuite struct {
	suite.Suite

	logger   zerolog.Logger
	wsConfig Config
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(WsControllerSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *WsControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.wsConfig = NewDefaultWebsocketConfig()
}

// TestSubscribeRequest tests the subscribe to topic flow.
// We emulate a request message from a client, and a response message from a controller.
func (s *WsControllerSuite) TestSubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{})

		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				<-done
			}).
			Return(nil).
			Once()

		request := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				ClientMessageID: uuid.New().String(),
				Action:          models.SubscribeAction,
			},
			Topic:     dp.BlocksTopic,
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

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.SubscribeMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				require.Equal(t, request.ClientMessageID, response.ClientMessageID)
				require.Equal(t, id.String(), response.SubscriptionID)

				return websocket.ErrCloseSent
			})

		s.expectCloseConnection(conn, done)
		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})

	s.T().Run("Parse and validate error", func(t *testing.T) {
		t.Parallel()

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

		done := make(chan struct{})
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

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
	})

	s.T().Run("Error creating data provider", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, _ := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("error creating data provider")).
			Once()

		done := make(chan struct{})
		s.expectSubscribeRequest(t, conn)

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

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
	})

	s.T().Run("Provider execution error", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProvider.On("ID").Return(uuid.New())
		// data provider might finish on its own or controller will close it via Close()
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

		done := make(chan struct{})
		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				defer close(done)

				response, ok := msg.(models.BaseMessageResponse)
				require.True(t, ok)
				require.False(t, response.Success)
				require.NotEmpty(t, response.Error)
				require.Equal(t, int(SubscriptionError), response.Error.Code)

				return websocket.ErrCloseSent
			})

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})
}

func (s *WsControllerSuite) TestUnsubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{})

		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				<-done
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				ClientMessageID: uuid.New().String(),
				Action:          models.UnsubscribeAction,
			},
			SubscriptionID: id.String(),
		}
		requestJson, err := json.Marshal(request)
		require.NoError(t, err)

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
				require.Equal(t, request.ClientMessageID, response.ClientMessageID)
				require.Equal(t, request.SubscriptionID, response.SubscriptionID)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})

	s.T().Run("Invalid subscription uuid", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{})

		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				<-done
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				ClientMessageID: uuid.New().String(),
				Action:          models.UnsubscribeAction,
			},
			SubscriptionID: "invalid-uuid",
		}
		requestJson, err := json.Marshal(request)
		require.NoError(t, err)

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
				require.Equal(t, request.ClientMessageID, response.ClientMessageID)
				require.Equal(t, int(InvalidArgument), response.Error.Code)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})

	s.T().Run("Unsubscribe from unknown subscription", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		done := make(chan struct{})

		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				<-done
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		request := models.UnsubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				ClientMessageID: uuid.New().String(),
				Action:          models.UnsubscribeAction,
			},
			SubscriptionID: uuid.New().String(),
		}
		requestJson, err := json.Marshal(request)
		require.NoError(t, err)

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

				require.Equal(t, request.ClientMessageID, response.ClientMessageID)
				require.Equal(t, int(NotFound), response.Error.Code)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
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

		done := make(chan struct{})

		id := uuid.New()
		topic := dp.BlocksTopic
		dataProvider.On("ID").Return(id)
		dataProvider.On("Topic").Return(topic)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()
		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {
				<-done
			}).
			Return(nil).
			Once()

		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		request := models.ListSubscriptionsMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{
				ClientMessageID: uuid.New().String(),
				Action:          models.ListSubscriptionsAction,
			},
		}
		requestJson, err := json.Marshal(request)
		require.NoError(t, err)

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
				require.Equal(t, request.ClientMessageID, response.ClientMessageID)
				require.Equal(t, 1, len(response.Subscriptions))
				require.Equal(t, id.String(), response.Subscriptions[0].ID)
				require.Equal(t, topic, response.Subscriptions[0].Topic)

				return websocket.ErrCloseSent
			}).
			Once()

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})
}

// TestSubscribeBlocks tests the functionality for streaming blocks to a subscriber.
func (s *WsControllerSuite) TestSubscribeBlocks() {
	s.T().Run("Stream one block", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
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

		done := make(chan struct{})
		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

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
				require.Equal(t, expectedBlock, actualBlock)

				return websocket.ErrCloseSent
			})

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})

	s.T().Run("Stream many blocks", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
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

		done := make(chan struct{})
		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

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
					require.Equal(t, expectedBlocks, actualBlocks)
					close(done)
					return websocket.ErrCloseSent
				}

				return nil
			}).
			Times(len(expectedBlocks))

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})
}

// TestConfigureKeepaliveConnection ensures that the WebSocket connection is configured correctly.
func (s *WsControllerSuite) TestConfigureKeepaliveConnection() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn := connmock.NewWebsocketConnection(t)
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil)

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		err := controller.configureKeepalive()
		s.Require().NoError(err, "configureKeepalive should not return an error")

		conn.AssertExpectations(t)
	})
}

func (s *WsControllerSuite) TestControllerShutdown() {
	s.T().Run("Keepalive routine failed", func(t *testing.T) {
		t.Parallel()

		conn := connmock.NewWebsocketConnection(t)
		conn.On("Close").Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil).Once()
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		// Mock keepalive to return an error
		done := make(chan struct{})
		conn.
			On("WriteControl", websocket.PingMessage, mock.Anything).
			Return(func(int, time.Time) error {
				close(done)
				return websocket.ErrCloseSent
			}).
			Once()

		conn.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				<-done
				return websocket.ErrCloseSent
			}).
			Once()

		controller.HandleConnection(context.Background())
		conn.AssertExpectations(t)
	})

	s.T().Run("Read routine failed", func(t *testing.T) {
		t.Parallel()

		conn := connmock.NewWebsocketConnection(t)
		conn.On("Close").Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil).Once()
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		conn.
			On("ReadJSON", mock.Anything).
			Return(func(_ interface{}) error {
				return websocket.ErrCloseSent //TODO: this should be assert.AnError and test should be rewritten
			}).
			Once()

		controller.HandleConnection(context.Background())
		conn.AssertExpectations(t)
	})

	s.T().Run("Write routine failed", func(t *testing.T) {
		t.Parallel()

		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, dataProviderFactory)

		dataProviderFactory.
			On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(dataProvider, nil).
			Once()

		id := uuid.New()
		dataProvider.On("ID").Return(id)
		// data provider might finish on its own or controller will close it via Close()
		dataProvider.On("Close").Return(nil).Maybe()

		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				controller.multiplexedStream <- unittest.BlockFixture()
			}).
			Return(nil).
			Once()

		done := make(chan struct{})
		msgID := s.expectSubscribeRequest(t, conn)
		s.expectSubscribeResponse(t, conn, msgID)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				close(done)
				return assert.AnError
			})

		s.expectCloseConnection(conn, done)

		controller.HandleConnection(context.Background())

		// Ensure all expectations are met
		conn.AssertExpectations(t)
		dataProviderFactory.AssertExpectations(t)
		dataProvider.AssertExpectations(t)
	})

	s.T().Run("Context cancelled", func(t *testing.T) {
		t.Parallel()

		conn := connmock.NewWebsocketConnection(t)
		conn.On("Close").Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil).Once()
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		ctx, cancel := context.WithCancel(context.Background())
		conn.On("ReadJSON", mock.Anything).Return(func(_ interface{}) error {
			<-ctx.Done()
			return websocket.ErrCloseSent
		}).Once()

		cancel()
		controller.HandleConnection(ctx)

		conn.AssertExpectations(t)
	})

	s.T().Run("Inactivity tracking", func(t *testing.T) {
		t.Parallel()

		conn := connmock.NewWebsocketConnection(t)
		conn.On("Close").Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil).Once()
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()

		factory := dpmock.NewDataProviderFactory(t)
		// Mock with short inactivity timeout for testing
		wsConfig := s.wsConfig

		wsConfig.InactivityTimeout = 50 * time.Millisecond
		controller := NewWebSocketController(s.logger, wsConfig, conn, factory)

		conn.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				// waiting more than InactivityTimeout to make sure that read message routine busy and do not return
				// an error before than inactivity tracker initiate shut down
				<-time.After(wsConfig.InactivityTimeout + 10)
				return websocket.ErrCloseSent
			}).
			Once()

		controller.HandleConnection(context.Background())
		time.Sleep(wsConfig.InactivityTimeout)

		conn.AssertExpectations(t)
	})
}

func (s *WsControllerSuite) TestKeepaliveRoutine() {
	s.T().Run("Successfully pings connection n times", func(t *testing.T) {
		conn := connmock.NewWebsocketConnection(t)
		conn.On("Close").Return(nil).Once()
		conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()
		conn.On("SetReadDeadline", mock.Anything).Return(nil)

		done := make(chan struct{})
		i := 0
		expectedCalls := 2
		conn.
			On("WriteControl", websocket.PingMessage, mock.Anything).
			Return(func(int, time.Time) error {
				if i == expectedCalls {
					close(done)
					return websocket.ErrCloseSent
				}

				i += 1
				return nil
			}).
			Times(expectedCalls + 1)

		conn.On("ReadJSON", mock.Anything).Return(func(_ interface{}) error {
			<-done
			return websocket.ErrCloseSent
		})

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)
		controller.HandleConnection(context.Background())

		conn.AssertExpectations(t)
	})

	s.T().Run("Error on write to connection", func(t *testing.T) {
		conn := connmock.NewWebsocketConnection(t)
		conn.
			On("WriteControl", websocket.PingMessage, mock.Anything).
			Return(websocket.ErrCloseSent). //TODO: change to assert.AnError and rewrite test
			Once()

		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := controller.keepalive(ctx)
		s.Require().Error(err)
		s.Require().ErrorIs(websocket.ErrCloseSent, err)

		conn.AssertExpectations(t)
	})

	s.T().Run("Context cancelled", func(t *testing.T) {
		conn := connmock.NewWebsocketConnection(t)
		factory := dpmock.NewDataProviderFactory(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, conn, factory)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel the context

		// Start the keepalive process with the context canceled
		err := controller.keepalive(ctx)
		s.Require().NoError(err)

		conn.AssertExpectations(t) // Should not invoke WriteMessage after context cancellation
	})
}

// TestMonitorInactivity verifies that monitorInactivity returns an error
// when the WebSocket connection has no subscriptions for the configured inactivity timeout.
func (s *WsControllerSuite) TestMonitorInactivity() {
	conn := connmock.NewWebsocketConnection(s.T())
	factory := dpmock.NewDataProviderFactory(s.T())

	// Mock with short inactivity timeout for testing
	wsConfig := s.wsConfig

	wsConfig.InactivityTimeout = 50 * time.Millisecond
	controller := NewWebSocketController(s.logger, wsConfig, conn, factory)

	err := controller.monitorInactivity(context.Background())
	s.Require().Error(err)
	s.Require().Equal(err, fmt.Errorf("no recent activity for %v", wsConfig.InactivityTimeout))

	conn.AssertExpectations(s.T())
}

// TestRateLimiter tests the rate-limiting functionality of the WebSocket controller.
//
// Test Steps:
// 1. Create a mock WebSocket connection with behavior for `SetWriteDeadline` and `WriteJSON`.
// 2. Configure the WebSocket controller with a rate limit of 2 responses per second.
// 3. Simulate sending messages to the `multiplexedStream` channel.
// 4. Collect timestamps of message writes to verify rate-limiting behavior.
// 5. Assert that all messages are processed and that the delay between messages respects the configured rate limit.
//
// The test ensures that:
// - The number of messages processed matches the total messages sent.
// - The delay between consecutive messages falls within the expected range based on the rate limit, with a tolerance of 5ms.
func (s *WsControllerSuite) TestRateLimiter() {
	s.T().Run("Enforces response rate limit", func(t *testing.T) {
		totalMessages := 5 // Number of messages to simulate.

		// Step 1: Create a mock WebSocket connection.
		conn := connmock.NewWebsocketConnection(t)
		conn.On("SetWriteDeadline", mock.Anything).Return(nil).Times(totalMessages)

		// Step 2: Configure the WebSocket controller with a rate limit.
		config := NewDefaultWebsocketConfig()
		config.MaxResponsesPerSecond = 2 // 2 messages per second.
		controller := NewWebSocketController(s.logger, config, conn, nil)

		// Step 3: Simulate sending messages to the controller's `multiplexedStream`.
		go func() {
			for i := 0; i < totalMessages; i++ {
				controller.multiplexedStream <- map[string]interface{}{
					"message": i,
				}
			}
			close(controller.multiplexedStream)
		}()

		// Step 4: Collect timestamps of message writes for verification.
		var timestamps []time.Time
		conn.On("WriteJSON", mock.Anything).Run(func(args mock.Arguments) {
			timestamps = append(timestamps, time.Now())
		}).Return(nil).Times(totalMessages)

		// Invoke the `writeMessages` method to process the stream.
		_ = controller.writeMessages(context.Background())

		// Step 5: Verify that all messages are processed.
		require.Len(t, timestamps, totalMessages, "All messages should be processed")

		// Calculate the expected delay between messages based on the rate limit.
		expectedDelay := time.Second / time.Duration(config.MaxResponsesPerSecond)
		const tolerance = 5 * time.Millisecond // Allow up to 5ms deviation.

		// Step 6: Assert that the delays respect the rate limit with tolerance.
		for i := 1; i < len(timestamps); i++ {
			delay := timestamps[i].Sub(timestamps[i-1])
			assert.GreaterOrEqual(t, delay, expectedDelay-tolerance, "Messages should respect the minimum rate limit")
			assert.LessOrEqual(t, delay, expectedDelay+tolerance, "Messages should respect the maximum rate limit")
		}
	})
}

// newControllerMocks initializes mock WebSocket connection, data provider, and data provider factory.
// The mocked functions are expected to be called in a case when a test is expected to reach WriteJSON function.
func newControllerMocks(t *testing.T) (*connmock.WebsocketConnection, *dpmock.DataProviderFactory, *dpmock.DataProvider) {
	conn := connmock.NewWebsocketConnection(t)
	conn.On("Close").Return(nil).Once()
	conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()
	conn.On("SetReadDeadline", mock.Anything).Return(nil)
	conn.On("SetWriteDeadline", mock.Anything).Return(nil)

	dataProvider := dpmock.NewDataProvider(t)
	factory := dpmock.NewDataProviderFactory(t)

	return conn, factory, dataProvider
}

// expectSubscribeRequest mocks the client's subscription request.
func (s *WsControllerSuite) expectSubscribeRequest(t *testing.T, conn *connmock.WebsocketConnection) string {
	request := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{
			ClientMessageID: uuid.New().String(),
			Action:          models.SubscribeAction,
		},
		Topic: dp.BlocksTopic,
	}
	requestJson, err := json.Marshal(request)
	require.NoError(t, err)

	// The very first message from a client is a request to subscribe to some topic
	conn.
		On("ReadJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			msg, ok := args.Get(0).(*json.RawMessage)
			require.True(t, ok)
			*msg = requestJson
		}).
		Return(nil).
		Once()

	return request.ClientMessageID
}

// expectSubscribeResponse mocks the subscription response sent to the client.
func (s *WsControllerSuite) expectSubscribeResponse(t *testing.T, conn *connmock.WebsocketConnection, msgId string) {
	conn.
		On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			response, ok := args.Get(0).(models.SubscribeMessageResponse)
			require.True(t, ok)
			require.Equal(t, msgId, response.ClientMessageID)
			require.Equal(t, true, response.Success)
		}).
		Return(nil).
		Once()
}

func (s *WsControllerSuite) expectCloseConnection(conn *connmock.WebsocketConnection, done <-chan struct{}) {
	// In the default case, no further communication is expected from the client.
	// We wait for the writer routine to signal completion, allowing us to close the connection gracefully
	conn.
		On("ReadJSON", mock.Anything).
		Return(func(msg interface{}) error {
			<-done
			return websocket.ErrCloseSent
		}).
		Once()

	s.expectKeepaliveRoutineShutdown(conn, done)
}

func (s *WsControllerSuite) expectKeepaliveRoutineShutdown(conn *connmock.WebsocketConnection, done <-chan struct{}) {
	// We use Maybe() because a test may finish faster than keepalive routine trigger WriteControl
	conn.
		On("WriteControl", websocket.PingMessage, mock.Anything).
		Return(func(int, time.Time) error {
			select {
			case <-done:
				return websocket.ErrCloseSent
			default:
				return nil
			}
		}).
		Maybe()
}
