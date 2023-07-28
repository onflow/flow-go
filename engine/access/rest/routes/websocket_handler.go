package routes

import (
	"net/http"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

// SubscribeHandlerFunc is a function that contains endpoint handling logic for subscribes,
// it fetches necessary resources and returns an error.
type SubscribeHandlerFunc func(
	r *request.Request,
	w http.ResponseWriter,

	logger zerolog.Logger,
	api state_stream.API,
	eventFilterConfig state_stream.EventFilterConfig,
	maxStreams int32,
	streamCount *atomic.Int32,
	errorHandler func(w http.ResponseWriter, err error, errorLogger zerolog.Logger),
	jsonResponse func(w http.ResponseWriter, code int, response interface{}, errLogger zerolog.Logger),
)

// WSHandler is websocket handler implementing custom websocket handler function and allows easier handling of errors and
// responses as it wraps functionality for handling error and responses outside of endpoint handling.
type WSHandler struct {
	*HttpHandler
	subscribeFunc SubscribeHandlerFunc

	api               state_stream.API
	eventFilterConfig state_stream.EventFilterConfig
	maxStreams        int32
	streamCount       atomic.Int32
}

func NewWSHandler(
	logger zerolog.Logger,
	subscribeFunc SubscribeHandlerFunc,
	chain flow.Chain,
	api state_stream.API,
	eventFilterConfig state_stream.EventFilterConfig,
	maxGlobalStreams uint32,
) *WSHandler {
	handler := &WSHandler{
		subscribeFunc:     subscribeFunc,
		api:               api,
		eventFilterConfig: eventFilterConfig,
		maxStreams:        int32(maxGlobalStreams),
		streamCount:       atomic.Int32{},
	}
	handler.HttpHandler = NewHttpHandler(logger, chain)

	return handler
}

// ServeHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	errLog := h.Logger.With().Str("request_url", r.URL.String()).Logger()

	err := h.VerifyRequest(w, r)
	if err != nil {
		return
	}
	decoratedRequest := request.Decorate(r, h.HttpHandler.Chain)

	h.subscribeFunc(decoratedRequest,
		w,
		errLog,
		h.api,
		h.eventFilterConfig,
		h.maxStreams,
		&h.streamCount,
		h.errorHandler,
		h.jsonResponse)

}
