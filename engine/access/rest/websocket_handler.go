package rest

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

// SubscribeHandlerFunc is a function that contains endpoint handling logic for subscribes,
// it fetches necessary resources and returns an error.
type SubscribeHandlerFunc func(
	r *request.Request,
	w http.ResponseWriter,
	h *state_stream.SubscribeHandler,
) (interface{}, error)

// WSHandler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type WSHandler struct {
	*HttpHandler
	*state_stream.SubscribeHandler
	subscribeFunc SubscribeHandlerFunc
}

func NewWSHandler(
	logger zerolog.Logger,
	subscribeFunc SubscribeHandlerFunc,
	chain flow.Chain,
	api state_stream.API,
	conf state_stream.EventFilterConfig,
	maxGlobalStreams uint32,
) *WSHandler {
	handler := &WSHandler{
		subscribeFunc: subscribeFunc,
	}
	handler.HttpHandler = NewHttpHandler(logger, chain)
	handler.SubscribeHandler = state_stream.NewSubscribeHandler(api, chain, conf, maxGlobalStreams)
	return handler
}

// ServerHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	errLog := h.Logger.With().Str("request_url", r.URL.String()).Logger()

	err := h.VerifyRequest(w, r)
	if err != nil {
		return
	}
	decoratedRequest := request.Decorate(r, h.HttpHandler.Chain)

	response, err := h.subscribeFunc(decoratedRequest, w, h.SubscribeHandler)
	if err != nil {
		h.errorHandler(w, err, errLog)
		return
	}

	// apply the select filter if any select fields have been specified
	response, err = util.SelectFilter(response, decoratedRequest.Selects())
	if err != nil {
		h.errorHandler(w, err, errLog)
		return
	}

	// write response to response stream
	h.jsonResponse(w, http.StatusOK, response, errLog)
}
