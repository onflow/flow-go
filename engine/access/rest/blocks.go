package rest

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"

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

// getBlocksByID gets blocks by provided ID or list of IDs.
func getBlocksByIDs(r *request, backend access.API, link LinkGenerator) (interface{}, error) {

	ids, err := r.ids()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blocks := make([]*generated.Block, len(ids))
	for i, id := range ids {
		block, err := getBlock(forID(&id), r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlocksByHeight(r *request, backend access.API, link LinkGenerator) (interface{}, error) {
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

	// if the query is /blocks?height=final or /blocks?height=sealed, lookup the last finalized or the last sealed block
	if len(heights) == 1 && (heights[0] == finalHeightQueryParam || heights[0] == sealedHeightQueryParam) {
		block, err := getBlock(forFinalized(heights[0]), r, backend, link)
		if err != nil {
			return nil, err
		}

		return []*generated.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if len(heights) > 0 {
		uintHeights, err := toHeights(heights)
		if err != nil {
			heightError := fmt.Errorf("invalid height specified: %v", err)
			return nil, NewBadRequestError(heightError)
		}

		blocks := make([]*generated.Block, len(uintHeights))
		for i, h := range uintHeights {
			block, err := getBlock(forHeight(h), r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}

		return blocks, nil
	}

	// support providing end height as "sealed" or "final"
	if endHeight == finalHeightQueryParam || endHeight == sealedHeightQueryParam {
		latest, err := backend.GetLatestBlock(r.Context(), endHeight == sealedHeightQueryParam)
		if err != nil {
			return nil, err
		}

		endHeight = fmt.Sprintf("%d", latest.Header.Height)
	}

	// lookup block by start and end height range
	start, err := toHeight(startHeight)
	if err != nil {
		heightError := fmt.Errorf("invalid start height: %w", err)
		return nil, NewBadRequestError(heightError)
	}
	end, err := toHeight(endHeight)
	if err != nil {
		heightError := fmt.Errorf("invalid end height %s: %v", endHeight, err)
		return nil, NewBadRequestError(heightError)
	}

	if start > end {
		err := fmt.Errorf("start height must be less than or equal to end height")
		return nil, NewBadRequestError(err)
	}

	if end-start > MaxAllowedHeights {
		err := fmt.Errorf("height interval exceeding maximum interval of %d", MaxAllowedHeights)
		return nil, NewBadRequestError(err)
	}

	blocks := make([]*generated.Block, 0)
	// start and end height inclusive
	for i := start; i <= end; i++ {
		block, err := getBlock(forHeight(i), r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(req *request, backend access.API, _ LinkGenerator) (interface{}, error) {

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

func getBlock(option blockProviderOption, req *request, backend access.API, link LinkGenerator) (*generated.Block, error) {
	// lookup block
	blkProvider := NewBlockProvider(backend, option)
	blk, err := blkProvider.getBlock(req.Context())
	if err != nil {
		return nil, err
	}

	// lookup execution result
	// (even if not specified as expandable, since we need the execution result ID to generate its expandable link)
	executionResult, err := backend.GetExecutionResultForBlockID(req.Context(), blk.ID())
	if err != nil {
		// handle case where execution result is not yet available
		if se, ok := status.FromError(err); ok {
			if se.Code() == codes.NotFound {
				return blockResponse(blk, nil, link, req.expandFields)
			}
		}
		return nil, err
	}

	return blockResponse(blk, executionResult, link, req.expandFields)
}

// blockProvider is a layer of abstraction on top of the backend access.API and provides a uniform way to
// look up a block or a block header either by ID or by height
type blockProvider struct {
	id      *flow.Identifier
	height  uint64
	latest  bool
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
			blkProvider.latest = true
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

func (blkProvider *blockProvider) getBlock(ctx context.Context) (*flow.Block, error) {
	if blkProvider.id != nil {
		blk, err := blkProvider.backend.GetBlockByID(ctx, *blkProvider.id)
		if err != nil {
			return nil, err
		}
		return blk, nil
	}

	if blkProvider.latest {
		blk, err := blkProvider.backend.GetLatestBlock(ctx, blkProvider.sealed)
		if err != nil {
			return nil, err
		}
		return blk, nil
	}

	blk, err := blkProvider.backend.GetBlockByHeight(ctx, blkProvider.height)
	if err != nil {
		return nil, err
	}
	return blk, nil
}
