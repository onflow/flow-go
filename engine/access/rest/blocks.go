package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/onflow/flow-go/model/flow"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByIDs(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		block, err := getBlockByID(id, r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlockByID(id flow.Identifier, req *requestDecorator, backend access.API, link LinkGenerator) (*generated.Block, StatusError) {
	var responseBlock = new(generated.Block)
	responseBlock.Expandable = new(generated.BlockExpandable)

	// if payload is to be expanded then lookup full block which contains both header and payload
	if req.expands(ExpandableFieldPayload) {
		header, payload, statusError := blockLookup(req.Context(), id, backend)
		if statusError != nil {
			return nil, statusError
		}
		responseBlock.Header = header
		responseBlock.Payload = payload
	} else {

		// else only lookup header and add expandable link for payload
		header, statusError := headerLookup(req.Context(), id, backend)
		if statusError != nil {
			return nil, statusError
		}
		responseBlock.Header = header
		responseBlock.Payload = nil

		var err error
		responseBlock.Expandable.Payload, err = link.PayloadLink(id)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
	}

	// if execution result is to be expanded, then lookup execution result else add expandable link for execution result
	if req.expands(ExpandableExecutionResult) {
		executionResult, err := executionResultLookup(req.Context(), id, backend, link)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
		responseBlock.ExecutionResult = executionResult
	} else {
		var err error
		responseBlock.Expandable.ExecutionResult, err = link.ExecutionResultLink(id)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
	}

	// add self link
	selfLink, err := selfLink(id, link.BlockLink)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	responseBlock.Links = selfLink

	// ship it
	return responseBlock, nil
}

// todo(sideninja) use functions from block lookup to support expanding etc
func getBlocksByHeight(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, StatusError) {
	height := r.getParam("height")
	startHeight := r.getParam("start_height")
	endHeight := r.getParam("end_height")

	// if both height and one or both of start and end height are provided
	if height != "" && (startHeight != "" || endHeight != "") {
		err := fmt.Errorf("can only provide either heights or start and end height range")
		return nil, NewBadRequestError(err.Error(), err)
	}

	// if neither height nor start and end height are provided
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
			return nil, blockHeightLookupError(height, err)
		}
		responseBlock = blockResponse(flowBlock, link)

		return responseBlock, nil
	}

	flowBlockHeader, err := backend.GetBlockHeaderByHeight(ctx, height)
	if err != nil {
		return nil, blockHeightLookupError(height, err)
	}
	responseBlock.Header = blockHeaderResponse(flowBlockHeader)
	responseBlock.Links = blockLink(flowBlockHeader.ID(), link)

	return responseBlock, nil
}

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(req *requestDecorator, backend access.API, _ LinkGenerator) (interface{}, StatusError) {

	id, err := req.id()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	_, payload, statusErr := blockLookup(req.Context(), id, backend)
	if err != nil {
		return nil, statusErr
	}

	return payload, nil
}

func blockLookup(ctx context.Context, id flow.Identifier, backend access.API) (*generated.BlockHeader, *generated.BlockPayload, StatusError) {
	flowBlock, err := backend.GetBlockByID(ctx, id)
	if err != nil {
		return nil, nil, idLookupError(id, "block", err)
	}
	return blockHeaderResponse(flowBlock.Header), blockPayloadResponse(flowBlock.Payload), nil
}

func headerLookup(ctx context.Context, id flow.Identifier, backend access.API) (*generated.BlockHeader, StatusError) {
	flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, idLookupError(id, "block header", err)
	}
	return blockHeaderResponse(flowBlockHeader), nil
}

func idLookupError(id flow.Identifier, entityType string, err error) StatusError {
	msg := fmt.Sprintf("%s with ID %s not found", entityType, id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}

// todo(sideninja) refactor and merge
func blockHeightLookupError(height uint64, err error) StatusError {
	msg := fmt.Sprintf("block with height %d not found", height)
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}
