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
	"github.com/onflow/flow-go/model/flow"
)

const ExpandableFieldPayload = "payload"
const ExpandableExecutionResult = "execution_result"

// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByID(
	r *requestDecorator,
	backend access.API,
	linkGenerator LinkGenerator,
	logger zerolog.Logger,
) (interface{}, StatusError) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		block, err := getBlockByID(id, r, backend, linkGenerator)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlockByID(id flow.Identifier, req *requestDecorator, backend access.API, linkGenerator LinkGenerator) (*generated.Block, StatusError) {
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
		responseBlock.Expandable.Payload, err = linkGenerator.PayloadLink(id)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
	}

	// if execution result is to be expanded, then lookup execution result else add expandable link for execution result
	if req.expands(ExpandableExecutionResult) {
		executionResult, err := executionResultLookup(req.Context(), id, backend, linkGenerator)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
		responseBlock.ExecutionResult = executionResult
	} else {
		var err error
		responseBlock.Expandable.ExecutionResult, err = linkGenerator.ExecutionResultLink(id)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
	}

	// add self link
	selfLink, err := selfLink(id, linkGenerator.BlockLink)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	responseBlock.Links = selfLink

	// ship it
	return responseBlock, nil
}

// getExecutionResultByID gets Execution Result payload by ID
func getExecutionResultByID(
	req *requestDecorator,
	backend access.API,
	linkGenerator LinkGenerator,
	_ zerolog.Logger,
) (interface{}, StatusError) {

	id, err := req.id()
	if err != nil {
		return nil, NewBadRequestError(err.Error(), err)
	}

	executionResult, err := executionResultLookup(req.Context(), id, backend, linkGenerator)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for execution result ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	return executionResult, nil
}

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(
	req *requestDecorator,
	backend access.API,
	_ LinkGenerator,
	_ zerolog.Logger,
) (interface{}, StatusError) {

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
		return nil, nil, lookupError(id, "block", err)
	}
	return blockHeaderResponse(flowBlock.Header), blockPayloadResponse(flowBlock.Payload), nil
}

func headerLookup(ctx context.Context, id flow.Identifier, backend access.API) (*generated.BlockHeader, StatusError) {
	flowBlockHeader, err := backend.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, lookupError(id, "block header", err)
	}
	return blockHeaderResponse(flowBlockHeader), nil
}

func executionResultLookup(ctx context.Context, id flow.Identifier, backend access.API, linkGenerator LinkGenerator) (*generated.ExecutionResult, StatusError) {
	executionResult, err := backend.GetExecutionResultForBlockID(ctx, id)
	if err != nil {
		return nil, lookupError(id, "execution result", err)
	}

	executionResultResp := executionResultResponse(executionResult)
	executionResultResp.Links, err = selfLink(executionResult.ID(), linkGenerator.ExecutionResultLink)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for execution result ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	return executionResultResp, nil
}

func lookupError(id flow.Identifier, entityType string, err error) StatusError {
	msg := fmt.Sprintf("%s with ID %s not found", entityType, id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}
