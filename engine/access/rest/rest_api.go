package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
)

const BlockIDCntLimit = 50

var MaxAllowedBlockIDsCnt = BlockIDCntLimit

// RestAPIHandler provides the implementation of each of the REST API
type APIHandler struct {
	backend *backend.Backend
	logger  zerolog.Logger
}

func NewRestAPIHandler(backend *backend.Backend, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		backend: backend,
		logger:  logger,
	}
}

func (restAPI *APIHandler) BlocksIdGet(w http.ResponseWriter, r *http.Request) {
	// create a logger for the request
	errorLogger := restAPI.logger.With().Str("request_url", r.URL.String()).Logger()

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(r)
	idParam := vars["id"]

	// gorilla mux retains opening and ending square brackets for ids
	idParam = strings.TrimSuffix(idParam, "]")
	idParam = strings.TrimPrefix(idParam, "[")

	ids := strings.Fields(idParam)

	blocks := make([]*generated.Block, len(ids))

	if len(ids) > MaxAllowedBlockIDsCnt {
		restAPI.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("at most %d Block IDs can be requested at a time", MaxAllowedBlockIDsCnt), errorLogger)
		return
	}

	for i, id := range ids {
		flowID, err := flow.HexStringToIdentifier(id)
		if err != nil {
			restAPI.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ID %s", id), errorLogger)
			return
		}

		flowBlock, err := restAPI.backend.GetBlockByID(r.Context(), flowID)
		if err != nil {
			// if error has GRPC code NotFound, the return HTTP NotFound error
			if status.Code(err) == codes.NotFound {
				restAPI.errorResponse(w, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", id), errorLogger)
				return
			}
			errorLogger.Error().Err(err).Str("block_id", id).Msg("failed to look up block")
			restAPI.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("failed to look up block with ID %s", id), errorLogger)
			return
		}
		blocks[i] = toBlock(flowBlock)
	}

	restAPI.jsonResponse(w, blocks, errorLogger)
}

func (restAPI *APIHandler) jsonResponse(w http.ResponseWriter, responsePayload interface{}, errorLogger zerolog.Logger) {
	encodedBlocks, err := json.Marshal(responsePayload)
	if err != nil {
		errorLogger.Error().Err(err).Msg("failed to encode response")
		restAPI.errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}

	_, err = w.Write(encodedBlocks)
	if err != nil {
		errorLogger.Error().Err(err).Msg("failed to write response")
		restAPI.errorResponse(w, http.StatusInternalServerError, "error generating response", errorLogger)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (restAPI *APIHandler) NotImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusNotImplemented)
}

// errorResponse sends an HTTP error response to the client with the given return code and a model error with the given
// response message in the response body
func (restAPI *APIHandler) errorResponse(w http.ResponseWriter, returnCode int, responseMessage string, logger zerolog.Logger) {
	w.WriteHeader(returnCode)
	modelError := generated.ModelError{
		Code:    int32(returnCode),
		Message: responseMessage,
	}
	encodedError, err := json.Marshal(modelError)
	if err != nil {
		logger.Error().Str("response_message", responseMessage).Msg("failed to json encode error message")
		return
	}
	_, err = w.Write(encodedError)
	if err != nil {
		logger.Error().Err(err).Msg("failed to send error response")
	}
}
