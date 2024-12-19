package websockets

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// ControllerSuite is a test suite for the WebSocket Controller.
type ControllerSuite struct {
	suite.Suite

	logger zerolog.Logger
	config Config

	connection          *connectionmock.WebsocketConnection
	dataProviderFactory *dpmock.DataProviderFactory

	streamApi    *streammock.API
	streamConfig backend.Config
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ControllerSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *ControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.config = NewDefaultWebsocketConfig()

	s.connection = connectionmock.NewWebsocketConnection(s.T())
	s.dataProviderFactory = dpmock.NewDataProviderFactory(s.T())

	s.streamApi = streammock.NewAPI(s.T())
	s.streamConfig = backend.Config{}
}

// TestSubscribeRequest tests the subscribe to topic flow.
// We emulate a request message from a client, and a response message from a controller.
func (s *ControllerSuite) TestSubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.config, conn, dataProviderFactory)

		dataProvider.
			On("Run").
			Run(func(args mock.Arguments) {}).
			Return(nil).
			Once()

		subscribeRequest := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
			Topic:              dp.BlocksTopic,
			Arguments:          nil,
		}

		// Simulate receiving the subscription request from the client
		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				requestMsg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				subscribeRequestMessage, err := json.Marshal(subscribeRequest)
				require.NoError(t, err)
				*requestMsg = subscribeRequestMessage
			}).
			Return(nil).
			Once()

		// Channel to signal the test flow completion
		done := make(chan struct{}, 1)

		// Simulate writing a successful subscription response back to the client
		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				response, ok := msg.(models.SubscribeMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				close(done) // Signal that response has been sent
				return websocket.ErrCloseSent
			}).Once()

		// Simulate client closing connection after receiving the response
		conn.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				<-done
				return websocket.ErrCloseSent
			}).Once()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		controller.HandleConnection(ctx)
	})
}

// TestSubscribeBlocks tests the functionality for streaming blocks to a subscriber.
func (s *ControllerSuite) TestSubscribeBlocks() {
	s.T().Run("Stream one block", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.config, conn, dataProviderFactory)

		// Simulate data provider write a block to the controller
		expectedBlock := unittest.BlockFixture()
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				controller.communicationChannel <- expectedBlock
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		s.expectSubscriptionRequest(conn, done)
		s.expectSubscriptionResponse(conn, true)

		// Expect a valid block to be passed to WriteJSON.
		// If we got to this point, the controller executed all its logic properly
		var actualBlock flow.Block
		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				block, ok := msg.(flow.Block)
				require.True(t, ok)
				actualBlock = block

				close(done)
				return websocket.ErrCloseSent
			}).Once()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		controller.HandleConnection(ctx)
		require.Equal(t, expectedBlock, actualBlock)
	})

	s.T().Run("Stream many blocks", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.config, conn, dataProviderFactory)

		// Simulate data provider writes some blocks to the controller
		expectedBlocks := unittest.BlockFixtures(100)
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				for _, block := range expectedBlocks {
					controller.communicationChannel <- *block
				}
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		s.expectSubscriptionRequest(conn, done)
		s.expectSubscriptionResponse(conn, true)

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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		controller.HandleConnection(ctx)
		require.Equal(t, expectedBlocks, actualBlocks)
	})
}

// newControllerMocks initializes mock WebSocket connection, data provider, and data provider factory.
// The mocked functions are expected to be called in a case when a test is expected to reach WriteJSON function.
func newControllerMocks(t *testing.T) (*connectionmock.WebsocketConnection, *dpmock.DataProviderFactory, *dpmock.DataProvider) {
	conn := connectionmock.NewWebsocketConnection(t)
	conn.On("Close").Return(nil).Once()
	conn.On("SetReadDeadline", mock.Anything).Return(nil).Once()
	conn.On("SetWriteDeadline", mock.Anything).Return(nil)
	conn.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()

	id := uuid.New()
	topic := dp.BlocksTopic
	dataProvider := dpmock.NewDataProvider(t)
	dataProvider.On("ID").Return(id)
	dataProvider.On("Close").Return(nil)
	dataProvider.On("Topic").Return(topic)

	factory := dpmock.NewDataProviderFactory(t)
	factory.
		On("NewDataProvider", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(dataProvider, nil).
		Once()

	return conn, factory, dataProvider
}

// expectSubscriptionRequest mocks the client's subscription request.
func (s *ControllerSuite) expectSubscriptionRequest(conn *connectionmock.WebsocketConnection, done <-chan struct{}) {
	requestMessage := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
		Topic:              dp.BlocksTopic,
	}

	// The very first message from a client is a request to subscribe to some topic
	conn.On("ReadJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			reqMsg, ok := args.Get(0).(*json.RawMessage)
			require.True(s.T(), ok)
			msg, err := json.Marshal(requestMessage)
			require.NoError(s.T(), err)
			*reqMsg = msg
		}).
		Return(nil).
		Once()

	// In the default case, no further communication is expected from the client.
	// We wait for the writer routine to signal completion, allowing us to close the connection gracefully
	conn.
		On("ReadJSON", mock.Anything).
		Return(func(msg interface{}) error {
			<-done
			return websocket.ErrCloseSent
		})
}

// expectSubscriptionResponse mocks the subscription response sent to the client.
func (s *ControllerSuite) expectSubscriptionResponse(conn *connectionmock.WebsocketConnection, success bool) {
	conn.On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			response, ok := args.Get(0).(models.SubscribeMessageResponse)
			require.True(s.T(), ok)
			require.Equal(s.T(), success, response.Success)
		}).
		Return(nil).
		Once()
}

// TestConfigureKeepaliveConnection ensures that the WebSocket connection is configured correctly.
func (s *ControllerSuite) TestConfigureKeepaliveConnection() {
	controller := s.initializeController()

	// Mock configureConnection to succeed
	s.mockConnectionSetup()

	// Call configureKeepalive and check for errors
	err := controller.configureKeepalive()
	s.Require().NoError(err, "configureKeepalive should not return an error")

	// Assert expectations
	s.connection.AssertExpectations(s.T())
}

// TestControllerShutdown ensures that HandleConnection shuts down gracefully when an error occurs.
func (s *ControllerSuite) TestControllerShutdown() {
	s.T().Run("keepalive routine failed", func(*testing.T) {
		controller := s.initializeController()

		// Mock configureConnection to succeed
		s.mockConnectionSetup()

		// Mock keepalive to return an error
		done := make(chan struct{}, 1)
		s.connection.On("WriteControl", websocket.PingMessage, mock.Anything).Return(func(int, time.Time) error {
			close(done)
			return websocket.ErrCloseSent
		}).Once()

		s.connection.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				<-done
				return websocket.ErrCloseSent
			}).
			Once()

		s.connection.On("Close").Return(nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		controller.HandleConnection(ctx)

		// Ensure all expectations are met
		s.connection.AssertExpectations(s.T())
	})

	s.T().Run("read routine failed", func(*testing.T) {
		controller := s.initializeController()
		// Mock configureConnection to succeed
		s.mockConnectionSetup()

		s.connection.
			On("ReadJSON", mock.Anything).
			Return(func(_ interface{}) error {
				return assert.AnError
			}).
			Once()

		s.connection.On("Close").Return(nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		controller.HandleConnection(ctx)

		// Ensure all expectations are met
		s.connection.AssertExpectations(s.T())
	})

	s.T().Run("write routine failed", func(*testing.T) {
		controller := s.initializeController()

		// Mock configureConnection to succeed
		s.mockConnectionSetup()
		blocksDataProvider := s.mockBlockDataProviderSetup(uuid.New())

		done := make(chan struct{}, 1)
		requestMessage := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
			Topic:              dp.BlocksTopic,
			Arguments:          nil,
		}
		msg, err := json.Marshal(requestMessage)
		s.Require().NoError(err)

		// Mocks `ReadJSON(v interface{}) error` which accepts an uninitialize interface that
		// receives the contents of the read message. This logic mocks that behavior, setting
		// the target with the value `msg`
		s.connection.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				reqMsg, ok := args.Get(0).(*json.RawMessage)
				s.Require().True(ok)
				*reqMsg = msg
			}).
			Return(nil).
			Once()

		s.connection.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				<-done
				return websocket.ErrCloseSent
			})

		s.connection.On("SetWriteDeadline", mock.Anything).Return(nil).Once()
		s.connection.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				close(done)
				return assert.AnError
			})
		s.connection.On("Close").Return(nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		controller.HandleConnection(ctx)

		// Ensure all expectations are met
		s.connection.AssertExpectations(s.T())
		s.dataProviderFactory.AssertExpectations(s.T())
		blocksDataProvider.AssertExpectations(s.T())
	})

	s.T().Run("context closed", func(*testing.T) {
		controller := s.initializeController()

		// Mock configureConnection to succeed
		s.mockConnectionSetup()

		s.connection.On("Close").Return(nil).Once()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		controller.HandleConnection(ctx)

		// Ensure all expectations are met
		s.connection.AssertExpectations(s.T())
	})
}

// TestKeepaliveHappyCase tests the behavior of the keepalive function.
func (s *ControllerSuite) TestKeepaliveHappyCase() {
	// Create a context for the test
	ctx := context.Background()

	controller := s.initializeController()
	s.connection.On("WriteControl", websocket.PingMessage, mock.Anything).Return(nil)

	// Start the keepalive process in a separate goroutine
	go func() {
		err := controller.keepalive(ctx)
		s.Require().NoError(err)
	}()

	// Use Eventually to wait for some ping messages
	expectedCalls := 3 // expected 3 ping messages for 30 seconds
	s.Require().Eventually(func() bool {
		return len(s.connection.Calls) == expectedCalls
	}, time.Duration(expectedCalls)*PongWait, 1*time.Second, "not all ping messages were sent")

	s.connection.On("Close").Return(nil).Once()
	controller.shutdownConnection()

	// Assert that the ping was sent
	s.connection.AssertExpectations(s.T())
}

// TestKeepaliveError tests the behavior of the keepalive function when there is an error in writing the ping.
func (s *ControllerSuite) TestKeepaliveError() {
	controller := s.initializeController()

	// Setup the mock connection with an error
	s.connection.On("WriteControl", websocket.PingMessage, mock.Anything).Return(assert.AnError).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedError := fmt.Errorf("failed to write ping message: %w", assert.AnError)
	// Start the keepalive process
	err := controller.keepalive(ctx)
	s.Require().Error(err)
	s.Require().Equal(expectedError, err)

	// Assert expectations
	s.connection.AssertExpectations(s.T())
}

// TestKeepaliveContextCancel tests the behavior of keepalive when the context is canceled before a ping is sent and
// no ping message is sent after the context is canceled.
func (s *ControllerSuite) TestKeepaliveContextCancel() {
	controller := s.initializeController()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel the context

	// Start the keepalive process with the context canceled
	err := controller.keepalive(ctx)
	s.Require().NoError(err)

	// Assert expectations
	s.connection.AssertExpectations(s.T()) // Should not invoke WriteMessage after context cancellation
}

// initializeController initializes the WebSocket controller.
func (s *ControllerSuite) initializeController() *Controller {
	return NewWebSocketController(s.logger, s.config, s.connection, s.dataProviderFactory)
}

// mockDataProviderSetup is a helper which mocks a blocks data provider setup.
func (s *ControllerSuite) mockBlockDataProviderSetup(id uuid.UUID) *dpmock.DataProvider {
	dataProvider := dpmock.NewDataProvider(s.T())
	dataProvider.On("ID").Return(id).Twice()
	dataProvider.On("Close").Return(nil).Once()
	dataProvider.On("Topic").Return(dp.BlocksTopic).Once()
	s.dataProviderFactory.On("NewDataProvider", mock.Anything, dp.BlocksTopic, mock.Anything, mock.Anything).
		Return(dataProvider, nil).Once()
	dataProvider.On("Run").Return(nil).Once()

	return dataProvider
}

// mockConnectionSetup is a helper which mocks connection setup for SetReadDeadline and SetPongHandler.
func (s *ControllerSuite) mockConnectionSetup() {
	s.connection.On("SetReadDeadline", mock.Anything).Return(nil).Once()
	s.connection.On("SetPongHandler", mock.AnythingOfType("func(string) error")).Return(nil).Once()
}
