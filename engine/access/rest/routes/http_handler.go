package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rest/models"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

const MaxRequestSize = 2 << 20 // 2MB

// HttpHandler is custom http handler implementing custom handler function.
// HttpHandler function allows easier handling of errors and responses as it
// wraps functionality for handling error and responses outside of endpoint handling.
type HttpHandler struct {
	Logger zerolog.Logger
	Chain  flow.Chain
}

func NewHttpHandler(
	logger zerolog.Logger,
	chain flow.Chain,
) *HttpHandler {
	return &HttpHandler{
		Logger: logger,
		Chain:  chain,
	}
}

// VerifyRequest function acts as a wrapper to each request providing common handling functionality
// such as logging, error handling
func (h *HttpHandler) VerifyRequest(w http.ResponseWriter, r *http.Request) error {
	// create a logger
	errLog := h.Logger.With().Str("request_url", r.URL.String()).Logger()

	// limit requested body size
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
	err := r.ParseForm()
	if err != nil {
		h.errorHandler(w, err, errLog)
		return err
	}
	return nil
}

func (h *HttpHandler) errorHandler(w http.ResponseWriter, err error, errorLogger zerolog.Logger) {
	// rest status type error should be returned with status and user message provided
	var statusErr models.StatusError
	if errors.As(err, &statusErr) {
		h.errorResponse(w, statusErr.Status(), statusErr.UserMessage(), errorLogger)
		return
	}

	// handle cadence errors
	cadenceError := fvmErrors.Find(err, fvmErrors.ErrCodeCadenceRunTimeError)
	if cadenceError != nil {
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
		if se.Code() == codes.Unavailable {
			msg := fmt.Sprintf("Failed to process request: %s", se.Message())
			h.errorResponse(w, http.StatusServiceUnavailable, msg, errorLogger)
			return
		}
	}

	// stop going further - catch all error
	msg := "internal server error"
	errorLogger.Error().Err(err).Msg(msg)
	h.errorResponse(w, http.StatusInternalServerError, msg, errorLogger)
}

// jsonResponse builds a JSON response and send it to the client
func (h *HttpHandler) jsonResponse(w http.ResponseWriter, code int, response interface{}, errLogger zerolog.Logger) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	// serialize response to JSON and handler errors
	encodedResponse, err := json.MarshalIndent(response, "", "\t")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errLogger.Error().Err(err).Str("response", string(encodedResponse)).Msg("failed to indent response")
		return
	}

	w.WriteHeader(code)
	// write response to response stream
	_, err = w.Write(encodedResponse)
	if err != nil {
		errLogger.Error().Err(err).Str("response", string(encodedResponse)).Msg("failed to write http response")
	}
}

// errorResponse sends an HTTP error response to the client with the given return code
// and a model error with the given response message in the response body
func (h *HttpHandler) errorResponse(
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
