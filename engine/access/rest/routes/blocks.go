package routes

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

// GetBlocksByIDs gets blocks by provided ID or list of IDs.
func GetBlocksByIDs(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockByIDsRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	blocks := make([]*models.Block, len(req.IDs))

	for i, id := range req.IDs {
		block, err := getBlock(forID(&id), r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

// GetBlocksByHeight gets blocks by height.
func GetBlocksByHeight(r *request.Request, backend access.API, link models.LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := getBlock(forFinalized(req.Heights[0]), r, backend, link)
		if err != nil {
			return nil, err
		}

		return []*models.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*models.Block, len(req.Heights))
		for i, h := range req.Heights {
			block, err := getBlock(forHeight(h), r, backend, link)
			if err != nil {
				return nil, err
			}
			blocks[i] = block
		}

		return blocks, nil
	}

	// support providing end height as "sealed" or "final"
	if req.EndHeight == request.FinalHeight || req.EndHeight == request.SealedHeight {
		latest, _, err := backend.GetLatestBlock(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Header.Height // overwrite special value height with fetched

		if req.StartHeight > req.EndHeight {
			return nil, models.NewBadRequestError(fmt.Errorf("start height must be less than or equal to end height"))
		}
	}

	blocks := make([]*models.Block, 0)
	// start and end height inclusive
	for i := req.StartHeight; i <= req.EndHeight; i++ {
		block, err := getBlock(forHeight(i), r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetBlockPayloadByID gets block payload by ID
func GetBlockPayloadByID(r *request.Request, backend access.API, _ models.LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockPayloadRequest()
	if err != nil {
		return nil, models.NewBadRequestError(err)
	}

	blkProvider := NewBlockProvider(backend, forID(&req.ID))
	blk, _, statusErr := blkProvider.getBlock(r.Context())
	if statusErr != nil {
		return nil, statusErr
	}

	var payload models.BlockPayload
	err = payload.Build(blk.Payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func getBlock(option blockProviderOption, req *request.Request, backend access.API, link models.LinkGenerator) (*models.Block, error) {
	// lookup block
	blkProvider := NewBlockProvider(backend, option)
	blk, blockStatus, err := blkProvider.getBlock(req.Context())
	if err != nil {
		return nil, err
	}

	// lookup execution result
	// (even if not specified as expandable, since we need the execution result ID to generate its expandable link)
	var block models.Block
	executionResult, err := backend.GetExecutionResultForBlockID(req.Context(), blk.ID())
	if err != nil {
		// handle case where execution result is not yet available
		if se, ok := status.FromError(err); ok {
			if se.Code() == codes.NotFound {
				err := block.Build(blk, nil, link, blockStatus, req.ExpandFields)
				if err != nil {
					return nil, err
				}
				return &block, nil
			}
		}
		return nil, err
	}

	err = block.Build(blk, executionResult, link, blockStatus, req.ExpandFields)
	if err != nil {
		return nil, err
	}
	return &block, nil
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

func forFinalized(queryParam uint64) blockProviderOption {
	return func(blkProvider *blockProvider) {
		switch queryParam {
		case request.SealedHeight:
			blkProvider.sealed = true
			fallthrough
		case request.FinalHeight:
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

func (blkProvider *blockProvider) getBlock(ctx context.Context) (*flow.Block, flow.BlockStatus, error) {
	if blkProvider.id != nil {
		blk, _, err := blkProvider.backend.GetBlockByID(ctx, *blkProvider.id)
		if err != nil { // unfortunately backend returns internal error status if not found
			return nil, flow.BlockStatusUnknown, models.NewNotFoundError(
				fmt.Sprintf("error looking up block with ID %s", blkProvider.id.String()), err,
			)
		}
		return blk, flow.BlockStatusUnknown, nil
	}

	if blkProvider.latest {
		blk, status, err := blkProvider.backend.GetLatestBlock(ctx, blkProvider.sealed)
		if err != nil {
			// cannot be a 'not found' error since final and sealed block should always be found
			return nil, flow.BlockStatusUnknown, models.NewRestError(http.StatusInternalServerError, "block lookup failed", err)
		}
		return blk, status, nil
	}

	blk, status, err := blkProvider.backend.GetBlockByHeight(ctx, blkProvider.height)
	if err != nil { // unfortunately backend returns internal error status if not found
		return nil, flow.BlockStatusUnknown, models.NewNotFoundError(
			fmt.Sprintf("error looking up block at height %d", blkProvider.height), err,
		)
	}
	return blk, status, nil
}
