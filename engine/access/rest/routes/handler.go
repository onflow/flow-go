package routes

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

// ApiHandlerFunc is a function that contains endpoint handling logic,
// it fetches necessary resources and returns an error or response model.
type ApiHandlerFunc func(
	r *request.Request,
	backend access.API,
	generator models.LinkGenerator,
) (interface{}, error)

// Handler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type Handler struct {
	*HttpHandler
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
) *Handler {
	handler := &Handler{
		backend:        backend,
		apiHandlerFunc: handlerFunc,
		linkGenerator:  generator,
		HttpHandler:    NewHttpHandler(logger, chain),
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
	decoratedRequest := request.Decorate(r, h.Chain)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
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
