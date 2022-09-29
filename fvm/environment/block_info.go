package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
)

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

type BlockInfo struct {
	tracer *Tracer
	meter  Meter

	blockHeader *flow.Header
	blocks      Blocks
}

func NewBlockInfo(
	tracer *Tracer,
	meter Meter,
	blockHeader *flow.Header,
	blocks Blocks,
) *BlockInfo {
	return &BlockInfo{
		tracer:      tracer,
		meter:       meter,
		blockHeader: blockHeader,
		blocks:      blocks,
	}
}

// GetCurrentBlockHeight returns the current block height.
func (info *BlockInfo) GetCurrentBlockHeight() (uint64, error) {
	defer info.tracer.StartExtensiveTracingSpanFromRoot(
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
func (info *BlockInfo) GetBlockAtHeight(
	height uint64,
) (
	runtime.Block,
	bool,
	error,
) {
	defer info.tracer.StartSpanFromRoot(trace.FVMEnvGetBlockAtHeight).End()

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

	header, err := info.blocks.ByHeightFrom(height, info.blockHeader)
	// TODO (ramtin): remove dependency on storage and move this if condition
	// to blockfinder
	if errors.Is(err, storage.ErrNotFound) {
		return runtime.Block{}, false, nil
	} else if err != nil {
		return runtime.Block{}, false, fmt.Errorf(
			"get block at height failed for height %v: %w", height, err)
	}

	return runtimeBlockFromHeader(header), true, nil
}
