package rest

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {
	ids, err := toIDs(vars["id"])
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		flowBlock, err := backend.GetBlockByID(r.Context(), id)
		if err != nil {
			msg := fmt.Sprintf("block with ID %s not found", id.String())
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
