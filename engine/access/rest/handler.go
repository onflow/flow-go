package rest

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// Validator provides validation for http requests
type Validator interface {
	Validate(r *http.Request) error
}

// ApiHandlerFunc is a function that contains endpoint handling logic,
// it fetches necessary resources and returns an error or response model.
type ApiHandlerFunc func(
	r *requestDecorator,
	backend access.API,
	generator LinkGenerator,
) (interface{}, error)

// Handler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type Handler struct {
	logger         zerolog.Logger
	backend        access.API
	linkGenerator  LinkGenerator
	apiHandlerFunc ApiHandlerFunc
	validator      Validator
}

func NewHandler(
	logger zerolog.Logger,
	backend access.API,
	handlerFunc ApiHandlerFunc,
	generator LinkGenerator,
	validator Validator,
) *Handler {
	return &Handler{
		logger:         logger,
		backend:        backend,
		apiHandlerFunc: handlerFunc,
		linkGenerator:  generator,
		validator:      validator,
	}
}

// ServerHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// create a logger
	errLog := h.logger.With().Str("request_url", r.URL.String()).Logger()

	err := h.validator.Validate(r)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error(), errLog) // todo(sideninja) limit message
		return
	}

	// create request decorator with parsed values
	decoratedRequest := newRequestDecorator(r)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
	if err != nil {
		// rest status type error should be returned with status and user message provided
		if e, ok := err.(StatusError); ok {
			h.errorResponse(w, e.Status(), e.UserMessage(), errLog)
			return
		}

		// handle grpc status error returned from the backend calls, we are forwarding the message to the client
		if se, ok := err.(interface {
			GRPCStatus() *status.Status
		}); ok {
			if se.GRPCStatus().Code() == codes.NotFound { // handle not found error with correct status
				h.errorResponse(w, http.StatusNotFound, se.GRPCStatus().Message(), errLog)
				return
			}
			if se.GRPCStatus().Code() == codes.InvalidArgument { // handle not valid errors with correct status
				h.errorResponse(w, http.StatusBadRequest, se.GRPCStatus().Message(), errLog)
				return
			}
		}

		// stop going further - catch all error
		h.errorResponse(w, http.StatusInternalServerError, err.Error(), errLog)
		return
	}

	// apply the select filter if any select fields have been specified
	selectFields := decoratedRequest.selects()
	if len(selectFields) > 0 {
		var err error
		response, err = SelectFilter(response, selectFields)
		if err != nil {
			h.errorResponse(w, http.StatusInternalServerError, err.Error(), errLog)
		}
	}

	// write response to response stream
	h.jsonResponse(w, response, errLog)
}

// jsonResponse builds a JSON response and send it to the client
func (h *Handler) jsonResponse(w http.ResponseWriter, response interface{}, logger zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// serialise response to JSON and handler errors
	encodedResponse, encErr := json.MarshalIndent(response, "", "\t")
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
	// create error response model
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
