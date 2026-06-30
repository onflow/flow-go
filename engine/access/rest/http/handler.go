package http

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

// ApiHandlerFunc is a function that contains endpoint handling logic,
// it fetches necessary resources and returns an error or response model.
type ApiHandlerFunc func(
	r *common.Request,
	backend access.API,
	generator models.LinkGenerator,
) (any, error)

// Handler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type Handler struct {
	*common.HttpHandler
	backend        access.API
	linkGenerator  models.LinkGenerator
	apiHandlerFunc ApiHandlerFunc
}

func NewHandler(
	logger zerolog.Logger,
	backend access.API,
	handlerFunc ApiHandlerFunc,
	generator models.LinkGenerator,
	chain flow.Chain,
	maxRequestSize int64,
	maxResponseSize int64,
) *Handler {
	handler := &Handler{
		backend:        backend,
		apiHandlerFunc: handlerFunc,
		linkGenerator:  generator,
		HttpHandler:    common.NewHttpHandler(logger, chain, maxRequestSize, maxResponseSize),
	}

	return handler
}

// ServerHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	errLog := h.Logger.With().Str("request_url", r.URL.String()).Logger()

	err := h.VerifyRequest(w, r)
	if err != nil {
		return
	}
	decoratedRequest := common.Decorate(r, h.Chain)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
	if err != nil {
		h.ErrorHandler(w, err, errLog)
		return
	}

	// apply the select filter if any select fields have been specified
	response, err = util.SelectFilter(response, decoratedRequest.Selects())
	if err != nil {
		h.ErrorHandler(w, err, errLog)
		return
	}

	// write response to response stream
	h.JsonResponse(w, http.StatusOK, response, errLog)
}
