package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/util"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
)

const MaxRequestSize = 2 << 20 // 2MB

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
	logger         zerolog.Logger
	backend        access.API
	linkGenerator  models.LinkGenerator
	apiHandlerFunc ApiHandlerFunc
}

func NewHandler(
	logger zerolog.Logger,
	backend access.API,
	handlerFunc ApiHandlerFunc,
	generator models.LinkGenerator,
) *Handler {
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
	// create a logger
	errLog := h.logger.With().Str("request_url", r.URL.String()).Logger()

	// limit requested body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
	err := r.ParseForm()
	if err != nil {
		h.errorHandler(w, err, errLog)
		return
	}

	// create request decorator with parsed values
	decoratedRequest := request.Decorate(r)

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

func (h *Handler) errorHandler(w http.ResponseWriter, err error, errorLogger zerolog.Logger) {
	// rest status type error should be returned with status and user message provided
	var statusErr StatusError
	if errors.As(err, &statusErr) {
		h.errorResponse(w, statusErr.Status(), statusErr.UserMessage(), errorLogger)
		return
	}

	// handle cadence errors
	var cadenceError *fvmErrors.CadenceRuntimeError
	if fvmErrors.As(err, &cadenceError) {
		msg := fmt.Sprintf("Cadence error: %s", cadenceError.Error())
		h.errorResponse(w, http.StatusBadRequest, msg, errorLogger)
		return
	}

	// handle grpc status error returned from the backend calls, we are forwarding the message to the client
	if se, ok := status.FromError(err); ok {
		if se.Code() == codes.NotFound {
			msg := fmt.Sprintf("Flow resource not found: %s", se.Message())
			h.errorResponse(w, http.StatusNotFound, msg, errorLogger)
			return
		}
		if se.Code() == codes.InvalidArgument {
			msg := fmt.Sprintf("Invalid Flow argument: %s", se.Message())
			h.errorResponse(w, http.StatusBadRequest, msg, errorLogger)
			return
		}
		if se.Code() == codes.Internal {
			msg := fmt.Sprintf("Invalid Flow request: %s", se.Message())
			h.errorResponse(w, http.StatusBadRequest, msg, errorLogger)
			return
		}
	}

	// stop going further - catch all error
	msg := "internal server error"
	errorLogger.Error().Err(err).Msg(msg)
	h.errorResponse(w, http.StatusInternalServerError, msg, errorLogger)
}

// jsonResponse builds a JSON response and send it to the client
func (h *Handler) jsonResponse(w http.ResponseWriter, code int, response interface{}, errLogger zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)

	// serialize response to JSON and handler errors
	encodedResponse, err := json.MarshalIndent(response, "", "\t")
	if err != nil {
		errLogger.Error().Err(err).Str("response", string(encodedResponse)).Msg("failed to indent response")
		return
	}

	// write response to response stream
	_, err = w.Write(encodedResponse)
	if err != nil {
		errLogger.Error().Err(err).Str("response", string(encodedResponse)).Msg("failed to write http response")
	}
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
	modelError := models.ModelError{
		Code:    int32(returnCode),
		Message: responseMessage,
	}
	h.jsonResponse(w, returnCode, modelError, logger)
}
