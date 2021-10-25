package swagger

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/swagger/generated"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
)

// RestAPIHandler provides the implementation of each of the REST API
type RestAPIHandler struct {
	backend *backend.Backend
	logger  zerolog.Logger
	encoder encoding.Encoder
}

func NewRestAPIHandler(backend *backend.Backend, logger zerolog.Logger) *RestAPIHandler {
	return &RestAPIHandler{
		backend: backend,
		logger:  logger,
		encoder: encoding.DefaultEncoder, //use the default JSON encoder
	}
}

func (restAPI *RestAPIHandler) BlocksIdGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	idParam, ok := vars["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// gorilla mux retains opening and ending square brackets for ids
	idParam = strings.TrimSuffix(idParam, "]")
	idParam = strings.TrimPrefix(idParam, "[")

	ids := strings.Split(idParam, ",")

	blocks := make([]*generated.Block, len(ids))

	for i, id := range ids {
		flowID, err := flow.HexStringToIdentifier(id)
		if err != nil {
			restAPI.errorResponse(w, r, err)
		}

		flowBlock, err := restAPI.backend.GetBlockByID(r.Context(), flowID)
		if err != nil {
			restAPI.errorResponse(w, r, err)
		}
		blocks[i] = toBlock(flowBlock)
	}

	encodedBlocks, err := restAPI.encoder.Encode(blocks)
	if err != nil {
		restAPI.errorResponse(w, r, err)
	}

	_, err = w.Write(encodedBlocks)
	if err != nil {
		restAPI.errorResponse(w, r, err)
	}
	w.WriteHeader(http.StatusOK)
}

func (restAPI *RestAPIHandler) errorResponse(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	encodedError, encodingErr := restAPI.encoder.Encode(err.Error())
	if encodingErr != nil {
		restAPI.logger.Error().Str("request_url", r.URL.String()).Err(err).Msg("failed to encode error")
		return
	}
	_, err = w.Write(encodedError)
	if err != nil {
		restAPI.logger.Err(err).Msg("failed to send error response")
	}
}
