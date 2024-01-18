package execution_test

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
)

type QueueAction struct {
	ToFetch     []*flow.Collection
	Executables []*entity.ExecutableBlock
}

type BlockQueue interface {
	OnBlock(*flow.Block) (*QueueAction, error)
	OnCollection(*flow.Collection) (*QueueAction, error)
	OnBlockExecuted(blockID flow.Identifier, height uint64, commit flow.StateCommitment) (*QueueAction, error)
}

type BlockQueues struct {
	sync.Mutex
	// when receiving a new block, adding it to the map, and add missing collections to the map
	blocks map[flow.Identifier]*entity.ExecutableBlock
	// a collection could be included in multiple blocks,
	// when a missing block is received, it might trigger multiple blocks to be executable, which
	// can be looked up by the map
	// when a block is executed, its collections should be removed from this map unless a collection
	// is still referenced by other blocks, which will eventually be removed when those blocks are
	// executed.
	collections map[flow.Identifier]map[flow.Identifier]*entity.ExecutableBlock

	// blockIDsByHeight is used to find next executable block.
	// when a block is executed, the next executable block must be a block with height = current block height + 1
	// if there are multiple blocks at the same height as current height + 1, then only those whose parent is the
	// current block could be executable (assuming their collections are ready), which can be checked by
	// the parentByBlockID map
	// the following map allows us to find the next executable block by height
	blockIDsByHeight map[uint64]map[flow.Identifier]struct{} // for finding next executable block
	parentByBlockID  map[flow.Identifier]flow.Identifier     // block - parent

}

type Uploader interface {
	Upload(*execution.ComputationResult) error
}

type Broadcaster interface {
	BroadcastExecutionReceipt(uint64, *flow.ExecutionReceipt) (bool, error)
}

type UploadAndBroadcast struct {
	uploader    Uploader
	broadcaster Broadcaster
}

var _ ExecutionNotifier = (*UploadAndBroadcast)(nil)

func (uab *UploadAndBroadcast) OnBlockExecuted(result *execution.ComputationResult) (string, error) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		err := uab.uploader.Upload(result)
		if err != nil {
			log.Err(err).Msg("error while uploading block")
			// continue processing. uploads should not block execution
		}

	}()

	broadcasted, err := uab.broadcaster.BroadcastExecutionReceipt(
		result.ExecutableBlock.Block.Header.Height, result.ExecutionReceipt)
	if err != nil {
		log.Err(err).Msg("critical: failed to broadcast the receipt")
	}

	return fmt.Sprintf("broadcasted: %v", broadcasted), nil
}

// var _ Ingestion = (*IngestionCore)(nil)

type IngestionCore struct {
	notifier ExecutionNotifier
}

type ExecutionNotifier interface {
	OnBlockExecuted(*execution.ComputationResult) (log string, err error)
}

type BlockExecutor interface {
	ExecuteBlock(*entity.ExecutableBlock) (*execution.ComputationResult, error)
}
