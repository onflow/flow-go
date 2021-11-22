package rest

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		block, err := getBlockByID(r.Context(), id, r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlockByID(
	ctx context.Context,
	id flow.Identifier,
	req *requestDecorator,
	backend access.API,
	link LinkGenerator,
) (*generated.Block, StatusError) {

	var responseBlock = new(generated.Block)
	if req.expands(ExpandableFieldPayload) {
		flowBlock, err := backend.GetBlockByID(ctx, id)
		if err != nil {
			return nil, blockLookupError(id, err)
		}
		responseBlock = blockResponse(flowBlock, link)
		return responseBlock, nil
	}

	flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, blockLookupError(id, err)
	}
	responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	responseBlock.Links = blockLink(id, link)

	return responseBlock, nil

	//if req.expands(ExpandableExecutionResult) {
	//	// lookup ER here and add to response
	//}
}

func blockLookupError(id flow.Identifier, err error) StatusError {
	msg := fmt.Sprintf("block with ID %s not found", id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}

	return NewRestError(http.StatusInternalServerError, msg, err)
}
