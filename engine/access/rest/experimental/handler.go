package experimental

import (
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access/backends/extended"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/model/flow"
)

// ApiHandlerFunc is the handler function signature for experimental API endpoints.
// It uses extended.API as the backend instead of access.API.
type ApiHandlerFunc func(r *common.Request, backend extended.API, link commonmodels.LinkGenerator) (interface{}, error)

// Handler wraps an ApiHandlerFunc with common HTTP handling (error handling, JSON responses).
type Handler struct {
	*common.HttpHandler
	backend        extended.API
	linkGenerator  commonmodels.LinkGenerator
	apiHandlerFunc ApiHandlerFunc
}

// NewHandler creates a new experimental Handler.
func NewHandler(
	logger zerolog.Logger,
	backend extended.API,
	handlerFunc ApiHandlerFunc,
	linkGenerator commonmodels.LinkGenerator,
	chain flow.Chain,
	maxRequestSize int64,
	maxResponseSize int64,
) *Handler {
	return &Handler{
		backend:        backend,
		linkGenerator:  linkGenerator,
		apiHandlerFunc: handlerFunc,
		HttpHandler:    common.NewHttpHandler(logger, chain, maxRequestSize, maxResponseSize),
	}
}

// ServeHTTP handles the request: verify, decorate, execute handler, write JSON response.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	errLog := h.Logger.With().Str("request_url", r.URL.String()).Logger()

	err := h.VerifyRequest(w, r)
	if err != nil {
		return
	}
	decoratedRequest := common.Decorate(r, h.Chain)

	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
	if err != nil {
		h.ErrorHandler(w, err, errLog)
		return
	}

	h.JsonResponse(w, http.StatusOK, response, errLog)
}
