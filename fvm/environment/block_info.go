package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	storageErr "github.com/onflow/flow-go/storage"
)

type BlockInfo interface {
	// GetCurrentBlockHeight returns the current block height.
	GetCurrentBlockHeight() (uint64, error)

	// GetBlockAtHeight returns the block at the given height.
	GetBlockAtHeight(
		height uint64,
	) (
		runtime.Block,
		bool,
		error,
	)
}

type ParseRestrictedBlockInfo struct {
	txnState storage.TransactionPreparer
	impl     BlockInfo
}

func NewParseRestrictedBlockInfo(
	txnState storage.TransactionPreparer,
	impl BlockInfo,
) BlockInfo {
	return ParseRestrictedBlockInfo{
		txnState: txnState,
		impl:     impl,
	}
}

func (info ParseRestrictedBlockInfo) GetCurrentBlockHeight() (uint64, error) {
	return parseRestrict1Ret(
		info.txnState,
		trace.FVMEnvGetCurrentBlockHeight,
		info.impl.GetCurrentBlockHeight)
}

func (info ParseRestrictedBlockInfo) GetBlockAtHeight(
	height uint64,
) (
	runtime.Block,
	bool,
	error,
) {
	return parseRestrict1Arg2Ret(
		info.txnState,
		trace.FVMEnvGetBlockAtHeight,
		info.impl.GetBlockAtHeight,
		height)
}

type BlockInfoParams struct {
	Blocks      Blocks
	BlockHeader *flow.Header
}

func DefaultBlockInfoParams() BlockInfoParams {
	return BlockInfoParams{
		Blocks:      nil,
		BlockHeader: nil,
	}
}

type blockInfo struct {
	tracer tracing.TracerSpan
	meter  Meter

	blockHeader *flow.Header
	blocks      Blocks
}

func NewBlockInfo(
	tracer tracing.TracerSpan,
	meter Meter,
	blockHeader *flow.Header,
	blocks Blocks,
) BlockInfo {
	return &blockInfo{
		tracer:      tracer,
		meter:       meter,
		blockHeader: blockHeader,
		blocks:      blocks,
	}
}

// GetCurrentBlockHeight returns the current block height.
func (info *blockInfo) GetCurrentBlockHeight() (uint64, error) {
	defer info.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvGetCurrentBlockHeight).End()

	err := info.meter.MeterComputation(
		ComputationKindGetCurrentBlockHeight,
		1)
	if err != nil {
		return 0, fmt.Errorf("get current block height failed: %w", err)
	}

	if info.blockHeader == nil {
		return 0, errors.NewOperationNotSupportedError("GetCurrentBlockHeight")
	}
	return info.blockHeader.Height, nil
}

// GetBlockAtHeight returns the block at the given height.
func (info *blockInfo) GetBlockAtHeight(
	height uint64,
) (
	runtime.Block,
	bool,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetBlockAtHeight).End()

	err := info.meter.MeterComputation(
		ComputationKindGetBlockAtHeight,
		1)
	if err != nil {
		return runtime.Block{}, false, fmt.Errorf(
			"get block at height failed: %w", err)
	}

	if info.blocks == nil {
		return runtime.Block{}, false, errors.NewOperationNotSupportedError(
			"GetBlockAtHeight")
	}

	if info.blockHeader != nil && height == info.blockHeader.Height {
		return runtimeBlockFromHeader(info.blockHeader), true, nil
	}

	if height+uint64(flow.DefaultTransactionExpiry) < info.blockHeader.Height {
		return runtime.Block{}, false, errors.NewBlockHeightOutOfRangeError(height)
	}

	header, err := info.blocks.ByHeightFrom(height, info.blockHeader)
	// TODO (ramtin): remove dependency on storage and move this if condition
	// to blockfinder
	if errors.Is(err, storageErr.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		return runtime.Block{}, false, fmt.Errorf(
			"get block at height failed for height %v: %w", height, err)
	}

	return runtimeBlockFromHeader(header), true, nil
}
