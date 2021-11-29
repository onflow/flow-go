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
func getBlocksByIDs(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		blkProvider := NewBlockProvider(backend, withID(&id))
		block, err := getBlock(blkProvider, r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlocksByHeight(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	heights := r.getQueryParams("height")
	startHeight := r.getQueryParam("start_height")
	endHeight := r.getQueryParam("end_height")

	// if both height and one or both of start and end height are provided
	if len(heights) > 0 && (startHeight != "" || endHeight != "") {
		err := fmt.Errorf("can only provide either heights or start and end height range")
		return nil, NewBadRequestError(err)
	}

	// if neither height nor start and end height are provided
	if len(heights) == 0 && (startHeight == "" || endHeight == "") {
		err := fmt.Errorf("must provide either heights or start and end height range")
		return nil, NewBadRequestError(err)
	}

	var blocks []*generated.Block

	if len(heights) > 0 {

		heights, err := toHeights(heights)
		if err != nil {
			return nil, NewBadRequestError(err)
		}

		blocks = make([]*generated.Block, len(heights))
		for i, h := range heights {
			blkProvider := NewBlockProvider(backend, withHeight(h))
			block, err := getBlock(blkProvider, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}
		return blocks, nil
	}

	if startHeight != "" && endHeight != "" {
		start, err := toHeight(startHeight)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		end, err := toHeight(endHeight)
		if err != nil {
			return nil, NewBadRequestError(err)
		}

		if start > end {
			err := fmt.Errorf("start height must be lower than end height")
			return nil, NewBadRequestError(err)
		}

		for i := start; i < end; i++ {
			blkProvider := NewBlockProvider(backend, withHeight(i))
			block, err := getBlock(blkProvider, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(req *requestDecorator, backend access.API, _ LinkGenerator) (interface{}, error) {

	id, err := req.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blkProvider := NewBlockProvider(backend, withID(&id))
	blk, statusErr := blkProvider.getBlock(req.Context())
	if statusErr != nil {
		return nil, statusErr
	}
	payload, err := blockPayloadResponse(blk.Payload)
	if err != nil {
		msg := fmt.Sprintf("failed to generate response for block payload for ID %s", id.String())
		return nil, NewRestError(http.StatusInternalServerError, msg, err)
	}
	return payload, nil
}

func getBlock(blkProvider *blockProvider, req *requestDecorator, backend access.API, link LinkGenerator) (*generated.Block, error) {
	var responseBlock = new(generated.Block)
	responseBlock.Expandable = new(generated.BlockExpandable)

	var id flow.Identifier
	// if payload is to be expanded then lookup full block which contains both header and payload
	if req.expands(ExpandableFieldPayload) {
		blk, statusErr := blkProvider.getBlock(req.Context())
		if statusErr != nil {
			return nil, statusErr
		}
		headerResponse := blockHeaderResponse(blk.Header)
		payloadResponse, err := blockPayloadResponse(blk.Payload)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
		responseBlock.Header, responseBlock.Payload = headerResponse, payloadResponse
		id = blk.ID()
	} else {

		// else only lookup header and add expandable link for payload
		header, statusErr := blkProvider.getHeader(req.Context())
		if statusErr != nil {
			return nil, statusErr
		}
		responseBlock.Header = blockHeaderResponse(header)
		responseBlock.Payload = nil
		id = header.ID()

		payload, err := link.PayloadLink(id)
		if err != nil {
			msg := fmt.Sprintf("failed to generate response for block ID %s", id.String())
			return nil, NewRestError(http.StatusInternalServerError, msg, err)
		}
		responseBlock.Expandable.Payload = payload
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

func idLookupError(id *flow.Identifier, entityType string, err error) StatusError {
	msg := fmt.Sprintf("%s with ID %s not found", entityType, id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}

// todo(sideninja) refactor and merge
func heightLookupError(height uint64, entityType string, err error) StatusError {
	msg := fmt.Sprintf("%s at height %d not found", entityType, height)
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}

// blockProvider is a layer of abstraction on top of the backend access.API and provides a uniform way to
// lookup a block or a block header either by ID or by height
type blockProvider struct {
	id      *flow.Identifier
	height  uint64
	backend access.API
}

type blockProviderOption func(blkProvider *blockProvider)

func withID(id *flow.Identifier) blockProviderOption {
	return func(blkProvider *blockProvider) {
		blkProvider.id = id
	}
}
func withHeight(height uint64) blockProviderOption {
	return func(blkProvider *blockProvider) {
		blkProvider.height = height
	}
}

func NewBlockProvider(backend access.API, options ...blockProviderOption) *blockProvider {
	blkProvider := &blockProvider{
		backend: backend,
	}

	for _, o := range options {
		o(blkProvider)
	}
	return blkProvider
}

func (blkProvider *blockProvider) getBlock(ctx context.Context) (*flow.Block, StatusError) {
	if blkProvider.id != nil {
		blk, err := blkProvider.backend.GetBlockByID(ctx, *blkProvider.id)
		if err != nil {
			return nil, idLookupError(blkProvider.id, "block", err)
		}
		return blk, nil
	}
	blk, err := blkProvider.backend.GetBlockByHeight(ctx, blkProvider.height)
	if err != nil {
		return nil, heightLookupError(blkProvider.height, "block", err)
	}
	return blk, nil
}

func (blkProvider *blockProvider) getHeader(ctx context.Context) (*flow.Header, StatusError) {
	if blkProvider.id != nil {
		header, err := blkProvider.backend.GetBlockHeaderByID(ctx, *blkProvider.id)
		if err != nil {
			return nil, idLookupError(blkProvider.id, "block", err)
		}
		return header, nil
	}
	header, err := blkProvider.backend.GetBlockHeaderByHeight(ctx, blkProvider.height)
	if err != nil {
		return nil, heightLookupError(blkProvider.height, "block", err)
	}
	return header, nil
}
