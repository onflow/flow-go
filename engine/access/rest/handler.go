package rest

import (
	"encoding/json"
	"errors"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"net/http"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// ApiHandlerFunc is a function that contains endpoint handling logic,
// it fetches necessary resources and returns an error or response model.
type ApiHandlerFunc func(
	r *request.Request,
	backend access.API,
	generator LinkGenerator,
) (interface{}, error)

type ApiValidatorFunc func(r *request.Request) error

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

// ServerHTTP function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling, request decorators
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger()

	// create request decorator with parsed values
	decoratedRequest := request.Decorate(r)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator)
	if err != nil {
		h.errorHandler(w, err, errorLogger)
		return
	}

	// apply the select filter if any select fields have been specified
	response, err = SelectFilter(response, decoratedRequest.Selects())
	if err != nil {
		h.errorHandler(w, err, errorLogger)
		return
	}

	// write response to response stream
	h.jsonResponse(w, response, errorLogger)
}

func (h *Handler) errorHandler(w http.ResponseWriter, err error, errorLogger zerolog.Logger) {
	// rest status type error should be returned with status and user message provided
	var statusErr StatusError
	if errors.As(err, &statusErr) {
		h.errorResponse(w, statusErr.Status(), statusErr.UserMessage(), errorLogger)
		return
	}

	// handle grpc status error returned from the backend calls, we are forwarding the message to the client
	if se, ok := status.FromError(err); ok {
		if se.Code() == codes.NotFound {
			h.errorResponse(w, http.StatusNotFound, se.Message(), errorLogger)
			return
		}
		if se.Code() == codes.InvalidArgument {
			h.errorResponse(w, http.StatusBadRequest, se.Message(), errorLogger)
			return
		}
	}

	// stop going further - catch all error
	msg := "internal server error"
	errorLogger.Error().Err(err).Msg(msg)
	h.errorResponse(w, http.StatusInternalServerError, msg, errorLogger)
}

// jsonResponse builds a JSON response and send it to the client
func (h *Handler) jsonResponse(w http.ResponseWriter, response interface{}, errLogger zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// serialise response to JSON and handler errors
	encodedResponse, err := json.MarshalIndent(response, "", "\t")
	if err != nil {
		h.errorHandler(w, err, errLogger)
		return
	}

	// write response to response stream
	_, err = w.Write(encodedResponse)
	if err != nil {
		h.errorHandler(w, err, errLogger)
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
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(returnCode)

	// create error response model
	modelError := generated.ModelError{
		Code:    int32(returnCode),
		Message: responseMessage,
	}
	encodedError, err := json.Marshal(modelError)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error().Str("response_message", responseMessage).Msg("failed to json encode error message")
		return
	}

	_, err = w.Write(encodedError)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error().Err(err).Msg("failed to send error response")
	}
}
