package websockets

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/common"
	dp "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Handler struct {
	*common.HttpHandler

	ctx                 irrecoverable.SignalerContext
	logger              zerolog.Logger
	websocketConfig     Config
	dataProviderFactory dp.DataProviderFactory
}

var _ http.Handler = (*Handler)(nil)

func NewWebSocketHandler(
	ctx irrecoverable.SignalerContext,
	logger zerolog.Logger,
	config Config,
	chain flow.Chain,
	maxRequestSize int64,
	dataProviderFactory dp.DataProviderFactory,
) *Handler {
	return &Handler{
		ctx:                 ctx,
		HttpHandler:         common.NewHttpHandler(logger, chain, maxRequestSize),
		websocketConfig:     config,
		logger:              logger,
		dataProviderFactory: dataProviderFactory,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.HttpHandler.Logger.With().Str("component", "websocket-handler").Logger()

	err := h.HttpHandler.VerifyRequest(w, r)
	if err != nil {
		// VerifyRequest sets the response error before returning
		logger.Debug().Err(err).Msg("error validating websocket request")
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
		h.HttpHandler.ErrorHandler(w, common.NewRestError(http.StatusInternalServerError, "webSocket upgrade error: ", err), logger)
		return
	}

	controller := NewWebSocketController(logger, h.websocketConfig, NewWebsocketConnection(conn), h.dataProviderFactory)
	controller.HandleConnection(h.ctx)
}
