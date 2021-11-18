package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
)

const expandableFieldPayload = "payload"
const expandableExecutionResult = "execution_result"

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	logger zerolog.Logger,
) (interface{}, StatusError) {

	expandFields, _ := middleware.GetFieldsToExpand(r)

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

type blockResponseFactory struct {
	expandBlockPayload bool
	expandExecutionResults bool
	selectFields map[string]bool
}

func newBlockResponseFactory(expandFields map[string]bool, selectFields map[string]bool) *blockResponseFactory {
	blkFactory := new(blockResponseFactory)
	blkFactory.expandBlockPayload = expandFields[expandableFieldPayload]
	blkFactory.expandExecutionResults = expandFields[expandableExecutionResult]
	blkFactory.selectFields = selectFields
	return blkFactory
}

func (blkRespFactory *blockResponseFactory) blockResponse(ctx context.Context, id flow.Identifier, backend access.API, linkGenerator *LinkGenerator) error{
	var responseBlock generated.Block
	if blkRespFactory.expandBlockPayload {
		flowBlock, err := backend.GetBlockByID(ctx, id)
		if err != nil {
			return err
		}
		responseBlock.Payload = blockPayloadResponse(flowBlock.Payload)
		responseBlock.Header = blockHeaderResponse(flowBlock.Header)
	} else {
		flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
		if err != nil {
			return err
		}
		responseBlock.Payload = nil
		responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	}
	if blkRespFactory.expandExecutionResults {
		// lookup ER here and add to response
	}


}
