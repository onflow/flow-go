package websockets

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	connectionmock "github.com/onflow/flow-go/engine/access/rest/websockets/mock"
)

type ControllerSuite struct {
	suite.Suite

	connection *connectionmock.WebsocketConnection
	controller *Controller
}

func TestControllerSuite(t *testing.T) {
	suite.Run(t, new(ControllerSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *ControllerSuite) SetupTest() {
	s.connection = connectionmock.NewWebsocketConnection(s.T())

	// Create the controller
	log := zerolog.New(zerolog.NewConsoleWriter())
	config := Config{}
	s.controller = &Controller{
		logger:               log,
		config:               config,
		conn:                 s.connection,
		communicationChannel: make(chan interface{}),
		errorChannel:         make(chan error, 1),
	}
}

// Helper function to start the keepalive process.
func (s *ControllerSuite) startKeepalive(ctx context.Context, expectedError error) {
	go func() {
		err := s.controller.keepalive(ctx)
		if expectedError != nil {
			s.Require().Error(err)
			s.Require().Equal(expectedError, err)
		} else {
			s.Require().NoError(err)
		}
	}()
}

// Helper function to setup mock behavior for SetWriteDeadline and WriteMessage.
func (s *ControllerSuite) setupMockConnection(writeMessageError error) {
	s.connection.On("SetWriteDeadline", mock.Anything).Return(nil).Once()
	s.connection.On("WriteMessage", websocket.PingMessage, mock.Anything).Return(writeMessageError).Once()
}

// Helper function to wait for expected mock calls.
func (s *ControllerSuite) waitForMockCalls(expectedCalls int, timeout time.Duration, interval time.Duration, errorMessage string) {
	require.Eventually(s.T(), func() bool {
		return len(s.connection.Calls) == expectedCalls
	}, timeout, interval, errorMessage)
}

// TestKeepaliveError tests the behavior of the keepalive function when there is an error in writing the ping.
func (s *ControllerSuite) TestKeepaliveError() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup the mock connection with an error
	expectedError := fmt.Errorf("failed to write ping message: %w", assert.AnError)
	s.setupMockConnection(assert.AnError)

	// Start the keepalive process
	s.startKeepalive(ctx, expectedError)

	// Wait for the ping message or timeout
	expectedCalls := 2
	s.waitForMockCalls(expectedCalls, PongWait*3/2, 1*time.Second, "ping message was not sent")

	// Assert expectations
	s.connection.AssertExpectations(s.T())
}

// TestKeepaliveContextCancel tests the behavior of keepalive when the context is canceled before a ping is sent and
// no ping message is sent after the context is canceled.
func (s *ControllerSuite) TestKeepaliveContextCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel the context

	// Start the keepalive process with the context canceled
	s.startKeepalive(ctx, context.Canceled)

	// Wait for the timeout to ensure no ping is sent
	time.Sleep(PongWait)

	// Assert expectations
	s.connection.AssertExpectations(s.T()) // Should not invoke WriteMessage after context cancellation
}
