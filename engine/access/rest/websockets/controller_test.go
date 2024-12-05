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
	"github.com/stretchr/testify/suite"

	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/unittest"
)

// ControllerSuite is a test suite for the WebSocket Controller.
type ControllerSuite struct {
	suite.Suite

	logger zerolog.Logger
	config Config

	connection          *connectionmock.WebsocketConnection
	dataProviderFactory *dpmock.DataProviderFactory
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ControllerSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *ControllerSuite) SetupTest() {
	s.logger = unittest.Logger()
	s.config = Config{}

	s.connection = connectionmock.NewWebsocketConnection(s.T())
	s.dataProviderFactory = dpmock.NewDataProviderFactory(s.T())
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

		s.connection.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				reqMsg, ok := args.Get(0).(*json.RawMessage)
				s.Require().True(ok)
				msg, err := json.Marshal(requestMessage)
				s.Require().NoError(err)
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
	dataProvider.On("ID").Return(id).Once()
	dataProvider.On("Close").Return(nil).Once()
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
