package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// Handler is custom http handler implementing custom handler function.
// Handler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type Handler struct {
	logger      zerolog.Logger
	backend     access.API
	route       *mux.Route
	method      string
	pattern     string
	name        string
	handlerFunc func(
		req Request,
		backend access.API,
	) (interface{}, StatusError)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger()

	// create request dto
	expanded, _ := middleware.GetFieldsToExpand(r)
	selected, _ := middleware.GetFieldsToSelect(r)

	request := Request{
		r:        r,
		context:  r.Context(),
		params:   mux.Vars(r),
		expand:   expanded,
		selected: selected,
		body:     r.Body,
		route:    h.route,
	}

	// execute handler function and check for error
	response, err := h.handlerFunc(request, h.backend)
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
	h.jsonResponse(w, response, selected, errorLogger)
}

// addToRouter adds handler to provided router
func (h *Handler) addToRouter(router *mux.Router) {
	router.
		Methods(h.method).
		Path(h.pattern).
		Name(h.name).
		Handler(h)

	h.route = router.Get(h.name)
}

func (h *Handler) jsonResponse(w http.ResponseWriter, response interface{}, selected []string, logger zerolog.Logger) {
	// serialise response to JSON and handler errors
	encodedResponse, err := json.Marshal(response)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to encode response")
		h.errorResponse(w, http.StatusInternalServerError, "error generating response", logger)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	_, writeErr := w.Write(encodedResponse)
	if writeErr != nil {
		h.logger.Error().Err(err).Msg("failed to write response")
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

	w.WriteHeader(returnCode)
	_, err = w.Write(encodedError)
	if err != nil {
		logger.Error().Err(err).Msg("failed to send error response")
	}
}

type Request struct {
	r        *http.Request
	context  context.Context
	params   map[string]string
	expand   []string
	selected []string
	body     io.ReadCloser
	route    *mux.Route
}

func (r *Request) getParam(name string) string {
	return r.params[name] // todo(sideninja) handle missing
}

func (r *Request) expands(name string) bool {
	for _, v := range r.expand {
		if v == name {
			return true
		}
	}
	return false
}

const payload = "payload"

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
	_ http.ResponseWriter,
	_ *http.Request,
	_ map[string]string,
	_ access.API,
	_ zerolog.Logger,
) (interface{}, StatusError) {
	return nil, NewRestError(http.StatusNotImplemented, "endpoint not implemented", nil)
}
