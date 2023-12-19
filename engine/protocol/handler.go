package protocol

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

type Handler struct {
	api                  API
	signerIndicesDecoder hotstuff.BlockSignerDecoder
}

// HandlerOption is used to hand over optional constructor parameters
type HandlerOption func(*Handler)

func NewHandler(api API, options ...HandlerOption) *Handler {
	h := &Handler{
		api:                  api,
		signerIndicesDecoder: &signature.NoopBlockSignerDecoder{},
	}
	for _, opt := range options {
		opt(h)
	}
	return h
}

func WithBlockSignerDecoder(signerIndicesDecoder hotstuff.BlockSignerDecoder) func(*Handler) {
	return func(handler *Handler) {
		handler.signerIndicesDecoder = signerIndicesDecoder
	}
}

func (h *Handler) GetNetworkParameters(
	ctx context.Context,
	_ *access.GetNetworkParametersRequest,
) (*access.GetNetworkParametersResponse, error) {
	params := h.api.GetNetworkParameters(ctx)

	return &access.GetNetworkParametersResponse{
		ChainId: string(params.ChainID),
	}, nil
}

func (h *Handler) GetNodeVersionInfo(
	ctx context.Context,
	request *access.GetNodeVersionInfoRequest,
) (*access.GetNodeVersionInfoResponse, error) {
	nodeVersionInfo, err := h.api.GetNodeVersionInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &access.GetNodeVersionInfoResponse{
		Info: &entities.NodeVersionInfo{
			Semver:          nodeVersionInfo.Semver,
			Commit:          nodeVersionInfo.Commit,
			SporkId:         nodeVersionInfo.SporkId[:],
			ProtocolVersion: nodeVersionInfo.ProtocolVersion,
		},
	}, nil
}

// GetLatestProtocolStateSnapshot returns the latest serializable Snapshot
func (h *Handler) GetLatestProtocolStateSnapshot(ctx context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	snapshot, err := h.api.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	return &access.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
	}, nil
}

// GetProtocolStateSnapshotByBlockID returns serializable Snapshot by blockID
func (h *Handler) GetProtocolStateSnapshotByBlockID(ctx context.Context, req *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	blockID := convert.MessageToIdentifier(req.GetBlockId())
	snapshot, err := h.api.GetProtocolStateSnapshotByBlockID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return &access.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
	}, nil
}

// GetProtocolStateSnapshotByHeight returns serializable Snapshot by block height
func (h *Handler) GetProtocolStateSnapshotByHeight(ctx context.Context, req *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	snapshot, err := h.api.GetProtocolStateSnapshotByHeight(ctx, req.GetBlockHeight())
	if err != nil {
		return nil, err
	}

	return &access.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
	}, nil
}

// GetLatestBlockHeader gets the latest sealed block header.
func (h *Handler) GetLatestBlockHeader(
	ctx context.Context,
	req *access.GetLatestBlockHeaderRequest,
) (*access.BlockHeaderResponse, error) {
	header, err := h.api.GetLatestBlockHeader(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header)
}

// GetBlockHeaderByHeight gets a block header by height.
func (h *Handler) GetBlockHeaderByHeight(
	ctx context.Context,
	req *access.GetBlockHeaderByHeightRequest,
) (*access.BlockHeaderResponse, error) {
	header, err := h.api.GetBlockHeaderByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *Handler) GetBlockHeaderByID(
	ctx context.Context,
	req *access.GetBlockHeaderByIDRequest,
) (*access.BlockHeaderResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, err
	}
	header, err := h.api.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header)
}

// GetLatestBlock gets the latest sealed block.
func (h *Handler) GetLatestBlock(
	ctx context.Context,
	req *access.GetLatestBlockRequest,
) (*access.BlockResponse, error) {
	block, err := h.api.GetLatestBlock(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse())
}

// GetBlockByHeight gets a block by height.
func (h *Handler) GetBlockByHeight(
	ctx context.Context,
	req *access.GetBlockByHeightRequest,
) (*access.BlockResponse, error) {
	block, err := h.api.GetBlockByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse())
}

// GetBlockByID gets a block by ID.
func (h *Handler) GetBlockByID(
	ctx context.Context,
	req *access.GetBlockByIDRequest,
) (*access.BlockResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, err
	}
	block, err := h.api.GetBlockByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse())
}

func (h *Handler) blockResponse(block *flow.Block, fullResponse bool) (*access.BlockResponse, error) {
	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(block.Header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	var msg *entities.Block
	if fullResponse {
		msg, err = convert.BlockToMessage(block, signerIDs)
		if err != nil {
			return nil, err
		}
	} else {
		msg = convert.BlockToMessageLight(block)
	}
	return &access.BlockResponse{
		Block: msg,
	}, nil
}

func (h *Handler) blockHeaderResponse(header *flow.Header) (*access.BlockHeaderResponse, error) {
	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	msg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, err
	}

	return &access.BlockHeaderResponse{
		Block: msg,
	}, nil
}
