package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

type ApiHandlerFunc func(
	r *requestDecorator,
	backend access.API,
	generator LinkGenerator,
	logger zerolog.Logger,
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

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger()

	decoratedRequest := newRequestDecorator(r)

	// execute handler function and check for error
	response, err := h.apiHandlerFunc(decoratedRequest, h.backend, h.linkGenerator, errorLogger)
	if err != nil {
		switch e := err.(type) {
		case StatusError: // todo(sideninja) try handle not found error.Code - grpc unwrap
			errorResponse(w, e.Status(), e.UserMessage(), errorLogger)
		default:
			errorResponse(w, http.StatusInternalServerError, e.Error(), errorLogger)
		}

		// stop going further
		return
	}

	// serialise response to JSON and handler errors
	encodedResponse, encErr := json.Marshal(response)
	if encErr != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
		errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}

	// write response to response stream

	_, writeErr := w.Write(encodedResponse)
	if writeErr != nil {
		h.logger.Error().Err(err).Msg("failed to write response")
		errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// errorResponse sends an HTTP error response to the client with the given return code
// and a model error with the given response message in the response body
func errorResponse(
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

	w.WriteHeader(returnCode)
	_, err = w.Write(encodedError)
	if err != nil {
		logger.Error().Err(err).Msg("failed to send error response")
	}
}

// jsonDecode provides safe JSON decoding with sufficient erro handling.
func jsonDecode(body io.ReadCloser, dst interface{}) error {
	// validate size

	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return NewBadRequestError(msg, err)

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			return NewBadRequestError(msg, err)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return NewBadRequestError(msg, err)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return NewBadRequestError(msg, err)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return NewBadRequestError(msg, err)

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return NewRestError(http.StatusRequestEntityTooLarge, msg, err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return NewBadRequestError(msg, err)
	}

	return nil
}

// NotImplemented handler returns an error explaining the endpoint is not yet implemented
func NotImplemented(
	_ *requestDecorator,
	_ access.API,
	_ LinkGenerator,
	_ zerolog.Logger,
) (interface{}, StatusError) {
	return nil, NewRestError(http.StatusNotImplemented, "endpoint not implemented", nil)
}
