package rest

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(req Request, backend access.API) (interface{}, StatusError) {
	ids, err := toIDs(req.getParam("id"))
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		if req.expands(payload) {
			flowBlock, err := backend.GetBlockByID(req.context, id)
			if err != nil {
				return nil, blockError(err, id)
			}
			blocks[i] = blockResponse(flowBlock)
			continue
		}

		flowBlock, err := backend.GetBlockHeaderByID(req.context, id)
		if err != nil {
			return nil, blockError(err, id)
		}
		blocks[i] = blockHeaderOnlyResponse(flowBlock)
	}

	return blocks, nil
}

func blockError(err error, id flow.Identifier) StatusError {
	msg := fmt.Sprintf("block with ID %s not found", id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}

	return NewRestError(http.StatusInternalServerError, msg, err)
}
