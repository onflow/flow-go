package rest

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"net/http"

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
func getBlocksByIDs(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockByIDsRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blocks := make([]*generated.Block, len(req.IDs))
	for i, id := range req.IDs {
		block, err := getBlock(forID(&id), r, backend, link)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}

	return blocks, nil
}

func getBlocksByHeight(r *request.Request, backend access.API, link LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	if req.FinalHeight || req.SealedHeight {
		block, err := getBlock(forFinalized(req.Heights[0]), r, backend, link)
		if err != nil {
			return nil, err
		}

		return []*generated.Block{block}, nil
	}

	// if the query is /blocks/height=1000,1008,1049...
	if req.HasHeights() {
		blocks := make([]*generated.Block, len(req.Heights))
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
		latest, err := backend.GetLatestBlock(r.Context(), req.EndHeight == request.SealedHeight)
		if err != nil {
			return nil, err
		}

		req.EndHeight = latest.Header.Height // overwrite special value height with fetched
	}

	blocks := make([]*generated.Block, 0)
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

// getBlockPayloadByID gets block payload by ID
func getBlockPayloadByID(r *request.Request, backend access.API, _ LinkGenerator) (interface{}, error) {
	req, err := r.GetBlockPayloadRequest()
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	blkProvider := NewBlockProvider(backend, forID(&req.ID))
	blk, statusErr := blkProvider.getBlock(r.Context())
	if statusErr != nil {
		return nil, statusErr
	}
	payload, err := blockPayloadResponse(blk.Payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func getBlock(option blockProviderOption, req *request.Request, backend access.API, link LinkGenerator) (*generated.Block, error) {
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
				return blockResponse(blk, nil, link, req.ExpandFields)
			}
		}
		return nil, err
	}

	return blockResponse(blk, executionResult, link, req.ExpandFields)
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

func (blkProvider *blockProvider) getBlock(ctx context.Context) (*flow.Block, error) {
	if blkProvider.id != nil {
		blk, err := blkProvider.backend.GetBlockByID(ctx, *blkProvider.id)
		if err != nil {
			return nil, idLookupError(blkProvider.id, "block", err)
		}
		return blk, nil
	}

	if blkProvider.latest {
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

// idLookupError adds ID to the error message
func idLookupError(id *flow.Identifier, entityType string, err error) StatusError {
	msg := fmt.Sprintf("error looking up %s with ID %s", entityType, id.String())
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}

// heightLookupError adds height to the error message
func heightLookupError(height uint64, entityType string, err error) StatusError {
	msg := fmt.Sprintf("error looking up %s at height %d", entityType, height)
	// if error has GRPC code NotFound, then return HTTP NotFound error
	if status.Code(err) == codes.NotFound {
		return NewNotFoundError(msg, err)
	}
	return NewRestError(http.StatusInternalServerError, msg, err)
}
