package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const BlockIDCntLimit = 50

var MaxAllowedBlockIDsCnt = BlockIDCntLimit

// Handlers provide collection of handlers used by the API server
type Handlers struct {
	backend access.API
	logger  zerolog.Logger
}

func NewHandlers(backend access.API, logger zerolog.Logger) *Handlers {
	return &Handlers{
		backend: backend,
		logger:  logger,
	}
}

func (h *Handlers) BlocksIdGet(w http.ResponseWriter, r *http.Request) {
	// create h logger for the request
	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(r)
	idParam := vars["id"]

	// gorilla mux retains opening and ending square brackets for ids
	idParam = strings.TrimSuffix(idParam, "]")
	idParam = strings.TrimPrefix(idParam, "[")

	ids := strings.Split(idParam, ",")

	if len(ids) > MaxAllowedBlockIDsCnt {
		h.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("at most %d Block IDs can be requested at a time", MaxAllowedBlockIDsCnt), errorLogger)
		return
	}

	blocks := make([]*generated.Block, len(ids))

	for i, id := range ids {
		flowID, err := toID(id)
		if err != nil {
			h.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ID %s", id), errorLogger)
			return
		}

		flowBlock, err := h.backend.GetBlockByID(r.Context(), flowID)
		if err != nil {
			// if error has GRPC code NotFound, the return HTTP NotFound error
			if status.Code(err) == codes.NotFound {
				h.errorResponse(w, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", id), errorLogger)
				return
			}
			errorLogger.Error().Err(err).Str("block_id", id).Msg("failed to look up block")
			h.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to look up block with ID %s", id), errorLogger)
			return
		}
		blocks[i] = blockResponse(flowBlock)
	}

	h.jsonResponse(w, blocks, errorLogger)
}

// GetTransactionByID gets a transaction by requested ID.
func (h *Handlers) GetTransactionByID(w http.ResponseWriter, r *http.Request) {
	errorLogger := h.logger.With().Str("request_url", r.URL.String()).Logger() // todo(sideninja) refactor this to be initialized for us

	vars := mux.Vars(r)
	idFromRequest := vars["id"]
	id, err := toID(idFromRequest)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid transaction ID", errorLogger)
		return
	}

	tx, err := h.backend.GetTransaction(r.Context(), id)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("transaction fetching error: %s", err.Error()), errorLogger)
		return
	}

	h.jsonResponse(w, transactionResponse(tx), errorLogger)
}

// CreateTransaction creates a new transaction from provided payload.
func (h *Handlers) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var txBody generated.TransactionsBody
	err := h.jsonDecode(r.Body, &txBody)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error(), h.logger)
		return
	}

	tx, err := toTransaction(&txBody)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error(), h.logger) // todo(sideninja) use refactor err func
		return
	}

	err = h.backend.SendTransaction(r.Context(), &tx)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error(), h.logger)
		return
	}

	h.jsonResponse(w, transactionResponse(&tx), h.logger)
}

func (h *Handlers) jsonResponse(w http.ResponseWriter, responsePayload interface{}, errorLogger zerolog.Logger) {
	encodedBlocks, err := json.Marshal(responsePayload)
	if err != nil {
		errorLogger.Error().Err(err).Msg("failed to encode response")
		h.errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}

	_, err = w.Write(encodedBlocks)
	if err != nil {
		errorLogger.Error().Err(err).Msg("failed to write response")
		h.errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) jsonDecode(body io.ReadCloser, dst interface{}) error {
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
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &badRequest{status: http.StatusBadRequest, msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &badRequest{status: http.StatusRequestEntityTooLarge, msg: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &badRequest{status: http.StatusBadRequest, msg: msg}
	}

	return nil
}

func (h *Handlers) NotImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusNotImplemented)
}

// errorResponse sends an HTTP error response to the client with the given return code and a model error with the given
// response message in the response body
func (h *Handlers) errorResponse(w http.ResponseWriter, returnCode int, responseMessage string, logger zerolog.Logger) {
	w.WriteHeader(returnCode)
	modelError := generated.ModelError{
		Code:    int32(returnCode),
		Message: responseMessage,
	}
	encodedError, err := json.Marshal(modelError)
	if err != nil {
		logger.Error().Err(err).Str("response_message", responseMessage).Msg("failed to json encode error message")
		return
	}
	_, err = w.Write(encodedError)
	if err != nil {
		logger.Error().Err(err).Msg("failed to send error response")
	}
}

type badRequest struct {
	status int
	msg    string
}

func (mr *badRequest) Error() string {
	return mr.msg
}
