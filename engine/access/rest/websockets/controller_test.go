package websockets

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	dpmock "github.com/onflow/flow-go/engine/access/rest/websockets/data_provider/mock"
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
	//s.logger = unittest.LoggerWithWriterAndLevel(os.Stdout, zerolog.DebugLevel)
	s.logger = unittest.Logger()
	s.wsConfig = NewDefaultWebsocketConfig()
	s.streamApi = streammock.NewAPI(s.T())
	s.streamConfig = backend.Config{}
}

func TestWsControllerSuite(t *testing.T) {
	suite.Run(t, new(WsControllerSuite))
}

func (s *WsControllerSuite) TestSubscribeRequest() {
	s.T().Run("Happy path", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, dataProviderFactory, conn)

		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {}).
			Once()

		requestMessage := models.SubscribeMessageRequest{
			BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
			Topic:              "blocks",
			Arguments:          nil,
		}

		conn.
			On("ReadJSON", mock.Anything).
			Run(func(args mock.Arguments) {
				reqMsg, ok := args.Get(0).(*json.RawMessage)
				require.True(t, ok)
				msg, err := json.Marshal(requestMessage)
				require.NoError(t, err)
				*reqMsg = msg
			}).
			Return(nil).
			Once()

		done := make(chan struct{}, 1)
		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				response, ok := msg.(models.SubscribeMessageResponse)
				require.True(t, ok)
				require.True(t, response.Success)
				close(done)
				return websocket.ErrCloseSent
			})

		conn.
			On("ReadJSON", mock.Anything).
			Return(func(interface{}) error {
				_, ok := <-done
				if !ok {
					return websocket.ErrCloseSent
				}
				return nil
			})

		controller.HandleConnection(context.Background())
	})
}

func (s *WsControllerSuite) TestSubscribeBlocks() {
	s.T().Run("Stream one block", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, dataProviderFactory, conn)

		// we want data provider to write some block to controller
		expectedBlock := unittest.BlockFixture()
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				controller.communicationChannel <- expectedBlock
			}).
			Once()

		done := make(chan struct{}, 1)
		var actualBlock flow.Block

		s.expectSubscriptionRequest(conn, done)
		s.expectSubscriptionResponse(conn, true)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				block := msg.(flow.Block)
				actualBlock = block

				close(done)
				return websocket.ErrCloseSent
			})

		controller.HandleConnection(context.Background())
		require.Equal(t, expectedBlock, actualBlock)
	})

	s.T().Run("Stream many blocks", func(t *testing.T) {
		conn, dataProviderFactory, dataProvider := newControllerMocks(t)
		controller := NewWebSocketController(s.logger, s.wsConfig, dataProviderFactory, conn)

		// we want data provider to write some block to controller
		expectedBlocks := unittest.BlockFixtures(100)
		dataProvider.
			On("Run", mock.Anything).
			Run(func(args mock.Arguments) {
				for _, block := range expectedBlocks {
					controller.communicationChannel <- *block
				}
			}).
			Once()

		done := make(chan struct{}, 1)
		actualBlocks := make([]*flow.Block, len(expectedBlocks))
		i := 0

		s.expectSubscriptionRequest(conn, done)
		s.expectSubscriptionResponse(conn, true)

		conn.
			On("WriteJSON", mock.Anything).
			Return(func(msg interface{}) error {
				block := msg.(flow.Block)
				actualBlocks[i] = &block
				i += 1

				if i == len(expectedBlocks) {
					close(done)
					return websocket.ErrCloseSent
				}

				return nil
			}).
			Times(len(expectedBlocks))

		controller.HandleConnection(context.Background())
		require.Equal(t, expectedBlocks, actualBlocks)
	})
}

func newControllerMocks(t *testing.T) (*connmock.WebsocketConnection, *dpmock.Factory, *dpmock.DataProvider) {
	conn := connmock.NewWebsocketConnection(t)
	conn.On("Close").Return(nil).Once()

	id := uuid.New()
	topic := "blocks"
	dataProvider := dpmock.NewDataProvider(t)
	dataProvider.On("ID").Return(id)
	dataProvider.On("Close").Return(nil)
	dataProvider.On("Topic").Return(topic)

	factory := dpmock.NewFactory(t)
	factory.
		On("NewDataProvider", mock.Anything, mock.Anything).
		Return(dataProvider).
		Once()

	return conn, factory, dataProvider
}

func (s *WsControllerSuite) expectSubscriptionRequest(conn *connmock.WebsocketConnection, done <-chan struct{}) {
	requestMessage := models.SubscribeMessageRequest{
		BaseMessageRequest: models.BaseMessageRequest{Action: "subscribe"},
		Topic:              "blocks",
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
			_, ok := <-done
			if !ok {
				return websocket.ErrCloseSent
			}
			return nil
		})
}

func (s *WsControllerSuite) expectSubscriptionResponse(conn *connmock.WebsocketConnection, success bool) {
	conn.On("WriteJSON", mock.Anything).
		Run(func(args mock.Arguments) {
			response, ok := args.Get(0).(models.SubscribeMessageResponse)
			require.True(s.T(), ok)
			require.Equal(s.T(), success, response.Success)
		}).
		Return(nil).
		Once()
}
