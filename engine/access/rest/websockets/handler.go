package websockets

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
)

type Handler struct {
	*common.BaseHttpHandler

	logger          zerolog.Logger
	websocketConfig *Config
	streamApi       state_stream.API
	streamConfig    backend.Config
}

var _ http.Handler = (*Handler)(nil)

func NewWebSocketHandler(logger zerolog.Logger, config *Config, chain flow.Chain, streamApi state_stream.API, streamConfig backend.Config) *Handler {
	return &Handler{
		BaseHttpHandler: common.NewHttpHandler(logger, chain),
		websocketConfig: config,
		logger:          logger,
		streamApi:       streamApi,
		streamConfig:    streamConfig,
	}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//TODO: change to accept topic instead of URL
	logger := h.BaseHttpHandler.Logger.With().Str("websocket_subscribe_url", r.URL.String()).Logger()

	err := h.BaseHttpHandler.VerifyRequest(w, r)
	if err != nil {
		// VerifyRequest sets the response error before returning
		logger.Warn().Err(err).Msg("error validating websocket request")
		return
	}

	upgrader := websocket.Upgrader{
		// allow all origins by default, operators can override using a proxy
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.BaseHttpHandler.ErrorHandler(w, common.NewRestError(http.StatusInternalServerError, "webSocket upgrade error: ", err), logger)
		return
	}

	ctx := context.Background()
	controller := NewWebSocketController(ctx, logger, h.websocketConfig, h.streamApi, h.streamConfig, conn)
	controller.HandleConnection()
}
