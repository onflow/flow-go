package find_inconsistent_result

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

var NoMissmatchFoundError = errors.New("No missmatch found")

var (
	flagDatadir   string
	flagEndHeight uint64
)

var Cmd = &cobra.Command{
	Use:   "find-inconsistent-result",
	Short: "find the first block that produces inconsistent results",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = Cmd.MarkPersistentFlagRequired("datadir")

	Cmd.Flags().Uint64Var(&flagEndHeight, "end-height", 0, "the last block height checks for result consistency")
}

func run(*cobra.Command, []string) {
	height, err := findFirstMismatch(flagDatadir, flagEndHeight)
	if err != nil {
		if errors.Is(err, NoMissmatchFoundError) {
			fmt.Printf("no mismatch found: %v\n", err)
		} else {
			fmt.Printf("fatal: %v\n", err)
		}
		return
	}

	fmt.Printf("first mismatch found at height %v\n", height)
}

func findFirstMismatch(datadir string, endHeight uint64) (uint64, error) {
	headers, results, state, db, err := createStorages(datadir)
	defer db.Close()
	if err != nil {
		return 0, fmt.Errorf("could not create storages: %v", err)
	}

	c := &checker{
		headers: headers,
		results: results,
		state:   state,
	}

	startHeight, err := findRootBlockHeight(state)
	if err != nil {
		return 0, fmt.Errorf("could not find root block height: %v", err)
	}

	if endHeight == 0 {
		lastExecutedSealed, err := findLastExecutedAndSealedHeight(state, db)
		if err != nil {
			return 0, fmt.Errorf("could not find last executed and sealed height: %v", err)
		}
		endHeight = lastExecutedSealed
	}

	mismatchHeight, err := c.FindFirstMismatchHeight(startHeight, endHeight)
	if err != nil {
		return 0, fmt.Errorf("could not find first mismatch: %v", err)
	}

	return mismatchHeight, nil
}

func createStorages(dir string) (storage.Headers, storage.ExecutionResults, protocol.State, *badger.DB, error) {
	db := common.InitStorage(dir)

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		return nil, nil, nil, db, fmt.Errorf("could not init protocol state: %v", err)
	}

	return storages.Headers, storages.Results, state, db, err
}

type checker struct {
	headers storage.Headers
	results storage.ExecutionResults
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

	sealedResultID, err := findSealedResultIDByBlockHeight(c.state, blockID)
	if err != nil {
		return false, fmt.Errorf("could not find sealed result for block %v: %w", blockID, err)
	}

	match := ownResultID == sealedResultID
	if match {
		fmt.Printf("height %v match: result %v\n", height, ownResultID)
	} else {
		fmt.Printf("height %v mismatch: own %v, sealed %v\n", height, ownResultID, sealedResultID)
	}

	return match, nil
}

func findRootBlockHeight(state protocol.State) (uint64, error) {
	root, err := state.Params().SealedRoot()
	if err != nil {
		return 0, fmt.Errorf("could not find root block height: %w", err)
	}
	return root.Height, nil
}

func findLastExecutedAndSealedHeight(state protocol.State, db *badger.DB) (uint64, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return 0, err
	}

	var blockID flow.Identifier
	var lastExecuted uint64
	err = db.View(procedure.GetHighestExecutedBlock(&lastExecuted, &blockID))
	if err != nil {
		return 0, err
	}

	// the last sealed executed is min(last_sealed, last_executed)
	if lastExecuted < lastSealed.Height {
		return lastExecuted, nil
	}
	return lastSealed.Height, nil
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

func findSealedResultIDByBlockHeight(state protocol.State, blockID flow.Identifier) (flow.Identifier, error) {
	result, _, err := state.AtBlockID(blockID).SealedResult()
	if err != nil {
		return flow.Identifier{}, err
	}

	return result.ID(), nil
}
