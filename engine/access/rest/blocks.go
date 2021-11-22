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
)

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

// getBlocksByIDs gets blocks by provided ID or collection of IDs.
func getBlocksByIDs(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

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

func getBlocksByHeights(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	height := r.getParam("height")
	startHeight := r.getParam("start_height")
	endHeight := r.getParam("end_height")

	if height != "" && (startHeight != "" || endHeight != "") {
		err := fmt.Errorf("can only provide either heights or start and end height range")
		return nil, NewBadRequestError(err.Error(), err)
	}
	if height == "" && (startHeight == "" || endHeight == "") {
		err := fmt.Errorf("must provide either heights or start and end height range")
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(height))

	if height != "" {
		heights, err := toHeights(height)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}

		for i, h := range heights {
			block, err := getBlockByHeight(r.Context(), h, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}
	}

	if startHeight != "" && endHeight != "" {
		start, err := toHeight(startHeight)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}
		end, err := toHeight(endHeight)
		if err != nil {
			return nil, NewBadRequestError(err.Error(), err)
		}

		if start > end {
			err := fmt.Errorf("start height must be lower than end height")
			return nil, NewBadRequestError(err.Error(), err)
		}

		for i := start; i < end; i++ {
			block, err := getBlockByHeight(r.Context(), i, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}
	}

	return blocks, nil
}

func getBlockByHeight(
	ctx context.Context,
	height uint64,
	req *requestDecorator,
	backend access.API,
	link LinkGenerator,
) (*generated.Block, StatusError) {
	var responseBlock = new(generated.Block)
	if req.expands(ExpandableFieldPayload) {
		flowBlock, err := backend.GetBlockByHeight(ctx, height)
		if err != nil {
			return nil, blockLookupError(string(height), err)
		}
		responseBlock = blockResponse(flowBlock, link)

		return responseBlock, nil
	}

	flowBlockHeader, err := backend.GetBlockHeaderByHeight(ctx, height)
	if err != nil {
		return nil, blockLookupError(string(height), err)
	}
	responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	responseBlock.Links = blockLink(flowBlockHeader.ID(), link)

	return responseBlock, nil
}

func getBlockByID(
	ctx context.Context,
	id flow.Identifier,
	req *requestDecorator,
	backend access.API,
	link LinkGenerator,
) (*generated.Block, StatusError) {

	var responseBlock = new(generated.Block)
	responseBlock.Expandable = new(generated.BlockExpandable)

	// if payload is to be expanded then lookup full block which contains both header and payload
	if req.expands(ExpandableFieldPayload) {
		flowBlock, err := backend.GetBlockByID(ctx, id)
		if err != nil {
			return nil, blockLookupError(id.String(), err)
		}
		responseBlock = blockResponse(flowBlock, link)

		return responseBlock, nil
	}

	flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, blockLookupError(id.String(), err)
	}
	responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	responseBlock.Links = blockLink(id, link)

	return responseBlock, nil

	//if req.expands(ExpandableExecutionResult) {
	//	// lookup ER here and add to response
	//}
}

func blockLookupError(id string, err error) StatusError {
	msg := fmt.Sprintf("block %s not found")
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}
