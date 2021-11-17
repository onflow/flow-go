package rest

import (
	"fmt"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

// getBlocksByID gets blocks by provided ID array.
func getBlocksByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	ids, err := toIDs(vars["id"])
	if err != nil {
		return nil, NewBadRequestError("invalid provided IDs", err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		flowBlock, err := backend.GetBlockByID(r.Context(), id)
		if err != nil {
			msg := fmt.Sprintf("block with id %s not found", id.String())
			// if error has GRPC code NotFound, then return HTTP NotFound error
			if status.Code(err) == codes.NotFound {
				return nil, NewNotFoundError(msg, err)
			}

			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}

		blocks[i] = blockResponse(flowBlock)
	}

	return blocks, nil
}
