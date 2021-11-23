package rest

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// ApiHandlerFunc is a function that contains endpoint handling logic,
// it fetches necessary resources and returns an error or response model.
type ApiHandlerFunc func(
	r *requestDecorator,
	backend access.API,
	generator LinkGenerator,
) (interface{}, StatusError)

// Handler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type Handler struct {
	logger         zerolog.Logger
	backend        access.API
	linkGenerator  LinkGenerator
	apiHandlerFunc ApiHandlerFunc
}

func NewHandler(logger zerolog.Logger, backend access.API, handlerFunc ApiHandlerFunc, generator LinkGenerator) *Handler {
	return &Handler{
		logger:         logger,
		backend:        backend,
		apiHandlerFunc: handlerFunc,
		linkGenerator:  generator,
	}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger()
	decoratedRequest := newRequestDecorator(r)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
	if err != nil {
		switch e := err.(type) {
		case StatusError: // todo(sideninja) try handle not found error.Code - grpc unwrap
			h.errorResponse(w, e.Status(), e.UserMessage(), errorLogger)
		default:
			h.errorResponse(w, http.StatusInternalServerError, e.Error(), errorLogger)
		}

		// stop going further
		return
	}

	// write response to response stream
	h.jsonResponse(w, response, errorLogger)
}

func (h *Handler) jsonResponse(w http.ResponseWriter, response interface{}, logger zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// serialise response to JSON and handler errors
	encodedResponse, encErr := json.Marshal(response)
	if encErr != nil {
		h.logger.Error().Err(encErr).Msg("failed to encode response")
		h.errorResponse(w, http.StatusInternalServerError, "error generating response", logger)
		return
	}

	// write response to response stream
	_, writeErr := w.Write(encodedResponse)
	if writeErr != nil {
		h.logger.Error().Err(encErr).Msg("failed to write response")
		h.errorResponse(w, http.StatusInternalServerError, "error generating response", logger)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// errorResponse sends an HTTP error response to the client with the given return code
// and a model error with the given response message in the response body
func (h *Handler) errorResponse(
	w http.ResponseWriter,
	returnCode int,
	responseMessage string,
	logger zerolog.Logger,
) {
	modelError := generated.ModelError{
		Code:    int32(returnCode),
		Message: responseMessage,
	}
	encodedError, err := json.Marshal(modelError)
	if err != nil {
		logger.Error().Str("response_message", responseMessage).Msg("failed to json encode error message")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(returnCode)
	_, err = w.Write(encodedError)
	if err != nil {
		logger.Error().Err(err).Msg("failed to send error response")
	}
}

// NotImplemented handler returns an error explaining the endpoint is not yet implemented
func NotImplemented(
	_ *requestDecorator,
	_ access.API,
	_ LinkGenerator,
) (interface{}, StatusError) {
	return nil, NewRestError(http.StatusNotImplemented, "endpoint not implemented", nil)
}
