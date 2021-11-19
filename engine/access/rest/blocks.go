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

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(
	w http.ResponseWriter,
	r *http.Request,
	vars map[string]string,
	backend access.API,
	linkGenerator LinkGenerator,
	logger zerolog.Logger,
) (interface{}, StatusError) {

	expandFields, _ := middleware.GetFieldsToExpand(r)
	selectFields, _ := middleware.GetFieldsToSelect(r)

	ids, err := toIDs(vars["id"])
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blockFactory := newBlockResponseFactory(expandFields, selectFields)

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		block, err := blockFactory.blockResponse(r.Context(), id, backend, linkGenerator)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

type blockResponseFactory struct {
	expandBlockPayload     bool
	expandExecutionResults bool
	selectFields           map[string]bool
}

func newBlockResponseFactory(expandFields map[string]bool, selectFields map[string]bool) *blockResponseFactory {
	blkFactory := new(blockResponseFactory)
	blkFactory.expandBlockPayload = expandFields[ExpandableFieldPayload]
	blkFactory.expandExecutionResults = expandFields[ExpandableExecutionResult]
	blkFactory.selectFields = selectFields
	return blkFactory
}

func (blkRespFactory *blockResponseFactory) blockResponse(ctx context.Context, id flow.Identifier, backend access.API, linkGenerator LinkGenerator) (*generated.Block, StatusError) {
	var responseBlock = new(generated.Block)
	if blkRespFactory.expandBlockPayload {
		flowBlock, err := backend.GetBlockByID(ctx, id)
		if err != nil {
			return nil, blockLookupError(id, err)
		}
		responseBlock.Payload = blockPayloadResponse(flowBlock.Payload)
		responseBlock.Header = blockHeaderResponse(flowBlock.Header)
	} else {
		flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
		if err != nil {
			return nil, blockLookupError(id, err)
		}
		responseBlock.Payload = nil
		responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	}
	//if blkRespFactory.expandExecutionResults {
	//	// lookup ER here and add to response
	//}

	blockLink, err := linkGenerator.BlockLink(id)
	if err != nil {
		msg := fmt.Sprintf("failed to generate respose for block ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}

	responseBlock.Links = new(generated.Links)
	responseBlock.Links.Self = blockLink

	return responseBlock, nil
}

func blockLookupError(id flow.Identifier, err error) StatusError {
	msg := fmt.Sprintf("block with ID %s not found", id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}

	return NewRestError(http.StatusInternalServerError, msg, err)
}
