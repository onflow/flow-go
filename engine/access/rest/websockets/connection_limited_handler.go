package websockets

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/module/limiters"
)

type ConnectionLimitedHandler struct {
	*common.HttpHandler

	log     zerolog.Logger
	handler http.Handler

	connections *limiters.ConcurrencyLimiter
}

func NewConnectionLimitedHandler(
	log zerolog.Logger,
	httpHandler *common.HttpHandler,
	handler http.Handler,
	limiter *limiters.ConcurrencyLimiter,
) *ConnectionLimitedHandler {
	return &ConnectionLimitedHandler{
		HttpHandler: httpHandler,
		log:         log,
		handler:     handler,
		connections: limiter,
	}
}

func (h *ConnectionLimitedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	allowed := h.connections.Allow(func() {
		h.handler.ServeHTTP(w, r)
	})
	if !allowed {
		h.HttpHandler.ErrorHandler(w, common.NewRestError(http.StatusTooManyRequests, "maximum number of connections reached", nil), h.log)
		return
	}
}
