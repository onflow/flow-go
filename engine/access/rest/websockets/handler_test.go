package websockets_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/rest/websockets"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	streammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	chainID = flow.Testnet
)

type WsHandlerSuite struct {
	suite.Suite

	logger  zerolog.Logger
	handler *websockets.Handler
}

func (s *WsHandlerSuite) SetupTest() {
	s.logger = unittest.Logger()
	wsConfig := websockets.NewDefaultWebsocketConfig()
	streamApi := streammock.NewAPI(s.T())
	streamConfig := backend.Config{}
	s.handler = websockets.NewWebSocketHandler(s.logger, wsConfig, chainID.Chain(), streamApi, streamConfig, 1024)
}

func TestWsHandlerSuite(t *testing.T) {
	suite.Run(t, new(WsHandlerSuite))
}

func ClientConnection(url string) (*websocket.Conn, *http.Response, error) {
	wsURL := "ws" + strings.TrimPrefix(url, "http")
	return websocket.DefaultDialer.Dial(wsURL, nil)
}

func (s *WsHandlerSuite) TestSubscribeRequest() {
	s.Run("Happy path", func() {
		server := httptest.NewServer(s.handler)
		defer server.Close()

		conn, _, err := ClientConnection(server.URL)
		defer conn.Close()
		require.NoError(s.T(), err)

		args := map[string]interface{}{
			"start_block_height": 10,
		}
		body := websockets.SubscribeMessageRequest{
			BaseMessageRequest: websockets.BaseMessageRequest{Action: "subscribe"},
			Topic:              "blocks",
			Arguments:          args,
		}
		bodyJSON, err := json.Marshal(body)
		require.NoError(s.T(), err)

		err = conn.WriteMessage(websocket.TextMessage, bodyJSON)
		require.NoError(s.T(), err)

		_, msg, err := conn.ReadMessage()
		require.NoError(s.T(), err)

		actualMsg := strings.Trim(string(msg), "\n\"\\ ")
		require.Equal(s.T(), "hello", actualMsg)

		_, msg, err = conn.ReadMessage()
		require.NoError(s.T(), err)

		actualMsg = strings.Trim(string(msg), "\n\"\\ ")
		require.Equal(s.T(), "world", actualMsg)
	})
}
