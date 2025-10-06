package find_inconsistent_result

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/latest"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var NoMissmatchFoundError = errors.New("No missmatch found")

var (
	flagDatadir     string
	flagStartHeight uint64
	flagEndHeight   uint64
)

var Cmd = &cobra.Command{
	Use:   "find-inconsistent-result",
	Short: "find the first block that produces inconsistent results",
	Run:   run,
}

func init() {
	common.InitDataDirFlag(Cmd, &flagDatadir)

	Cmd.Flags().Uint64Var(&flagEndHeight, "end-height", 0, "the last block height checks for result consistency")
	Cmd.Flags().Uint64Var(&flagStartHeight, "start-height", 0, "the first block height checks for result consistency")
}

func run(*cobra.Command, []string) {
	lockManager := storage.MakeSingletonLockManager()
	err := findFirstMismatch(flagDatadir, flagStartHeight, flagEndHeight, lockManager)
	if err != nil {
		if errors.Is(err, NoMissmatchFoundError) {
			fmt.Printf("no mismatch found: %v\n", err)
		} else {
			fmt.Printf("fatal: %v\n", err)
		}
	}
}

func findFirstMismatch(datadir string, startHeight, endHeight uint64, lockManager lockctx.Manager) error {
	fmt.Printf("initializing database\n")
	return common.WithStorage(datadir, func(db storage.DB) error {
		headers, results, seals, state, err := createStorages(db, lockManager)
		if err != nil {
			return fmt.Errorf("could not create storages: %v", err)
		}

		c := &checker{
			headers: headers,
			results: results,
			seals:   seals,
			state:   state,
		}

		if startHeight == 0 {
			startHeight = findRootBlockHeight(state)
		}

		if endHeight == 0 {
			endHeight, err = latest.LatestSealedAndExecutedHeight(state, db)
			if err != nil {
				return fmt.Errorf("could not find last executed and sealed height: %v", err)
			}
		}

		fmt.Printf("finding mismatch result between heights %v and %v\n", startHeight, endHeight)

		mismatchHeight, err := c.FindFirstMismatchHeight(startHeight, endHeight)
		if err != nil {
			return fmt.Errorf("could not find first mismatch: %v", err)
		}

		fmt.Printf("first mismatch found at block %v\n", mismatchHeight)

		blockID, err := findBlockIDByHeight(headers, mismatchHeight)
		if err != nil {
			return fmt.Errorf("could not find block id for height %v: %v", mismatchHeight, err)
		}

		fmt.Printf("mismatching block %v (id: %v)\n", mismatchHeight, blockID)

		return nil
	})
}

func createStorages(db storage.DB, lockManager lockctx.Manager) (
	storage.Headers, storage.ExecutionResults, storage.Seals, protocol.State, error) {
	storages := common.InitStorages(db)
	state, err := common.OpenProtocolState(lockManager, db, storages)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not open protocol state: %v", err)
	}

	return storages.Headers, storages.Results, storages.Seals, state, nil
}

type checker struct {
	headers storage.Headers
	results storage.ExecutionResults
	seals   storage.Seals
	state   protocol.State
}

func (c *checker) FindFirstMismatchHeight(startHeight uint64, endHeight uint64) (uint64, error) {
	low := startHeight
	high := endHeight
	firstMismatch := endHeight + 1 // Initialize to a value outside the range

	for low <= high {
		mid := low + (high-low)/2
		match, err := c.CompareAtHeight(mid)
		if err != nil {
			return 0, err
		}

		if !match {
			// Found a mismatch, update the first mismatch and search the lower half
			firstMismatch = mid
			high = mid - 1
		} else {
			// No mismatch, search the upper half
			low = mid + 1
		}
	}

	if firstMismatch > endHeight {
		// No mismatch found within the range
		return 0, fmt.Errorf("no mismatch found between heights %v and %v: %w", startHeight, endHeight, NoMissmatchFoundError)
	}

	return firstMismatch, nil
}

func (c *checker) CompareAtHeight(height uint64) (bool, error) {
	blockID, err := findBlockIDByHeight(c.headers, height)
	if err != nil {
		return false, fmt.Errorf("could not find block id for height %v: %w", height, err)
	}

	ownResultID, err := findOwnResultIDByBlockID(c.results, blockID)
	if err != nil {
		return false, fmt.Errorf("could not find own result for block %v: %w", blockID, err)
	}

	sealedResultID, err := findSealedResultIDByBlockHeight(c.seals, blockID)
	if err != nil {
		return false, fmt.Errorf("could not find sealed result for block %v: %w", blockID, err)
	}

	match := ownResultID == sealedResultID
	if match {
		fmt.Printf("block %v (id: %v) match: result %v\n", height, blockID, ownResultID)
	} else {
		fmt.Printf("block %v (id: %v) mismatch: own %v, sealed %v\n", height, blockID, ownResultID, sealedResultID)
	}

	return match, nil
}

func findRootBlockHeight(state protocol.State) uint64 {
	return state.Params().SealedRoot().Height
}

func findBlockIDByHeight(headers storage.Headers, height uint64) (flow.Identifier, error) {
	return headers.BlockIDByHeight(height)
}

func findOwnResultIDByBlockID(results storage.ExecutionResults, blockID flow.Identifier) (flow.Identifier, error) {
	result, err := results.ByBlockID(blockID)
	if err != nil {
		return flow.Identifier{}, err
	}
	return result.ID(), nil
}

func findSealedResultIDByBlockHeight(seals storage.Seals, blockID flow.Identifier) (flow.Identifier, error) {
	seal, err := seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return flow.Identifier{}, err
	}

	return seal.ResultID, nil
}
