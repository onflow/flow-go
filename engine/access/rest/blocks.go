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

const (
	ExpandableFieldPayload    = "payload"
	ExpandableExecutionResult = "execution_result"
	sealedHeightQueryParam    = "sealed"
	finalHeightQueryParam     = "final"
	startHeightQueryParam     = "start_height"
	endHeightQueryParam       = "end_height"
	heightQueryParam          = "height"
)
// getBlocksByID gets blocks by provided ID or collection of IDs.
func getBlocksByIDs(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		blkProvider := NewBlockProvider(backend, forID(&id))
		block, err := getBlock(blkProvider, r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlocksByHeight(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	heights := r.getQueryParams(heightQueryParam)
	startHeight := r.getQueryParam(startHeightQueryParam)
	endHeight := r.getQueryParam(endHeightQueryParam)

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
		// look up block by heights

		blocks = make([]*generated.Block, len(heights))

		// if the query is /blocks?height=final or /blocks?height=sealed, lookup the last finalized or the last sealed block
		if heights[0] == finalHeightQueryParam || heights[0] == sealedHeightQueryParam {
			blkProvider := NewBlockProvider(backend, forFinalized(heights[0]))
			block, err := getBlock(blkProvider, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[0] = block
			return blocks, nil
		}

		// if the query is /blocks/height=1000,1008,1049...
		heights, err := toHeights(heights)
		if err != nil {
			return nil, NewBadRequestError(err)
		}

		blocks = make([]*generated.Block, len(heights))
		for i, h := range heights {
			blkProvider := NewBlockProvider(backend, forHeight(h))
			block, err := getBlock(blkProvider, r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}
		return blocks, nil
	}

	// lookup block by start and end height range
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
		blkProvider := NewBlockProvider(backend, forHeight(i))
		block, err := getBlock(blkProvider, r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(req *requestDecorator, backend access.API, _ LinkGenerator) (interface{}, error) {

	id, err := req.id()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blkProvider := NewBlockProvider(backend, forID(&id))
	blk, statusErr := blkProvider.getBlock(req.Context())
	if statusErr != nil {
		return nil, statusErr
	}
	payload, err := blockPayloadResponse(blk.Payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func getBlock(blkProvider *blockProvider, req *requestDecorator, backend access.API, link LinkGenerator) (*generated.Block, error) {
	var responseBlock = new(generated.Block)
	responseBlock.Expandable = new(generated.BlockExpandable)

	// lookup block
	blk, statusErr := blkProvider.getBlock(req.Context())
	if statusErr != nil {
		return nil, statusErr
	}

	// lookup execution result
	// (even if not specified as expandable, since we need the execution result ID to generate its expandable link)
	executionResult, err := backend.GetExecutionResultForBlockID(req.Context(), blk.ID())
	if err != nil {
		return nil, err
	}

	return blockResponse(blk, executionResult, link, req.expandFields)
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
	final   bool
	sealed  bool
	backend access.API
}

type blockProviderOption func(blkProvider *blockProvider)

func forID(id *flow.Identifier) blockProviderOption {
	return func(blkProvider *blockProvider) {
		blkProvider.id = id
	}
}
func forHeight(height uint64) blockProviderOption {
	return func(blkProvider *blockProvider) {
		blkProvider.height = height
	}
}

func forFinalized(queryParam string) blockProviderOption {
	return func(blkProvider *blockProvider) {
		switch queryParam {
		case sealedHeightQueryParam:
			blkProvider.sealed = true
			fallthrough
		case finalHeightQueryParam:
			blkProvider.final = true
		}
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

	if blkProvider.final {
		blk, err := blkProvider.backend.GetLatestBlock(ctx, blkProvider.sealed)
		if err != nil {
			// cannot be a 'not found' error since final and sealed block should always be found
			return nil, NewRestError(http.StatusInternalServerError, "block lookup failed", err)
		}
		return blk, nil
	}

	blk, err := blkProvider.backend.GetBlockByHeight(ctx, blkProvider.height)
	if err != nil {
		return nil, heightLookupError(blkProvider.height, "block", err)
	}
	return blk, nil
}
