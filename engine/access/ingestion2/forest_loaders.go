package ingestion2

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type ResultEntry struct {
	result *flow.ExecutionResult
	header *flow.Header
}

// TODO: what happens when sealing is halted for an extended period of time?
// * if we continue indexing data, we may end up with a large amount of data in memory
// * if we don't, we will stop tracking data. maybe this is OK?

// TODO: do I need to prune when a certified block is orphaned?

// ForestLoader is used to load execution results into the ResultsForest, maintaining the max size
//
// There are 2 modes of operation:
// - Load from sealed data from storage -> the loader is reading sealed results from storage and adding to the forest
// - Load from network -> the loader is accepting new results from the network and adding to the forest
//
// The loader must be able to switch back and forth between the modes to handle times when ingestion
// or sealing is slow/delayed. After catching back up, it should seamlessly continue to accept new results
type ForestLoader struct {
	forest           *ResultsForest
	maxSealedResults uint

	state protocol.State // used to access the  protocol state

	blocks            storage.Blocks
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	executionResults  storage.ExecutionResults

	latestPersistedSealedResult flow.Identifier
}

func NewForestLoader(forest *ResultsForest, latestPersistedSealedResult flow.Identifier, maxSealedResults uint) *ForestLoader {
	return &ForestLoader{
		forest:                      forest,
		latestPersistedSealedResult: latestPersistedSealedResult,
		maxSealedResults:            maxSealedResults,
	}
}

// TODO: need to handle the transition from loading sealed data to unsealed data in a way that ensures
// no data is missed
// Maybe we can start loading data from the live network as soon as we finish loading sealed data
// then we may get some duplicated data pushed in, but that's OK since it's ignored
func (l *ForestLoader) Run(ctx context.Context) error {
	// first, load the latest persisted sealed result
	re, err := l.getLatestPersistedSealedResult()
	if err != nil {
		return fmt.Errorf("could not get latest persisted sealed result: %w", err)
	}

	// TODO: need a way to indicate that this data is already persisted and does not need to be started
	err = l.forest.AddResult(re.result, re.header, true)
	if err != nil {
		return fmt.Errorf("could not add latest persisted sealed result to forest: %w", err)
	}

	// next, load all sealed results.
	// only load as many results as can be concurrently started, and continue adding until all sealed
	// results are loaded.
	previousResultID := re.result.ID()
	previousHeader := re.header
	for {
		re, ok, err := l.getNextSealedResult(previousResultID, previousHeader.Height)
		if err != nil {
			return fmt.Errorf("could not get next sealed result: %w", err)
		}
		if !ok {
			break // no more sealed results
		}

		err = l.forest.AddResult(re.result, re.header, true)
		if err != nil {
			return fmt.Errorf("could not add sealed result to forest: %w", err)
		}

		previousResultID = re.result.ID()
		previousHeader = re.header

		if l.forest.Size() >= l.maxSealedResults {
			// TODO: add backpressure
		}
	}

	// finally, load all unsealed results in a single batch
	finalizedHeader, err := l.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized header: %w", err)
	}

	entries, err := l.getUnsealedResults(previousHeader, finalizedHeader)
	if err != nil {
		return fmt.Errorf("could not get unsealed results: %w", err)
	}

	for _, re := range entries {
		err := l.forest.AddResult(re.result, re.header, false)
		if err != nil {
			return fmt.Errorf("could not add result %s to forest: %w", re.result.ID(), err)
		}
	}

	return nil
}

func (l *ForestLoader) getLatestPersistedSealedResult() (*ResultEntry, error) {
	latestPersistedResult, err := l.executionResults.ByID(l.latestPersistedSealedResult)
	if err != nil {
		return nil, fmt.Errorf("could not get latest persisted result: %w", err)
	}

	header, err := l.headers.ByBlockID(latestPersistedResult.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get header for latest persisted result (blockID: %s): %w", latestPersistedResult.BlockID, err)
	}

	return &ResultEntry{
		result: latestPersistedResult,
		header: header,
	}, nil
}

func (l *ForestLoader) getNextSealedResult(previousResultID flow.Identifier, previousHeight uint64) (*ResultEntry, bool, error) {
	sealed, err := l.state.Sealed().Head()
	if err != nil {
		return nil, false, fmt.Errorf("could not get sealed header: %w", err)
	}

	nextHeight := previousHeight + 1
	if nextHeight > sealed.Height {
		return nil, false, nil
	}

	header, err := l.headers.ByHeight(nextHeight)
	if err != nil {
		return nil, false, fmt.Errorf("could not get header for block height %d: %w", nextHeight, err)
	}

	// note: this index is only available for sealed blocks
	result, err := l.executionResults.ByBlockID(header.ID())
	if err != nil {
		return nil, false, fmt.Errorf("could not get execution result for block %s: %w", header.ID(), err)
	}

	if result.PreviousResultID != previousResultID {
		return nil, false, fmt.Errorf("execution result %s does not descend from previous sealed result %s", result.PreviousResultID, previousResultID)
	}

	return &ResultEntry{
		result: result,
		header: header,
	}, true, nil
}

func (l *ForestLoader) getUnsealedResults(
	previousHeader, finalizedHeader *flow.Header,
) ([]*ResultEntry, error) {
	// walk block by block, finding all results referenced in the block results

	entries := make([]*ResultEntry, 0)

	// first, get all results from finalized blocks
	nextHeight := previousHeader.Height + 1
	for ; nextHeight <= finalizedHeader.Height; nextHeight++ {
		finalizedEntries, err := l.getResultsFromFinalizedBlock(nextHeight)
		if err != nil {
			return nil, fmt.Errorf("could not get finalized results for block height %d: %w", nextHeight, err)
		}
		entries = append(entries, finalizedEntries...)
	}

	unfinalizedEntries, err := l.getResultsFromUnfinalizedBlocks(finalizedHeader.ID())
	if err != nil {
		return nil, fmt.Errorf("could not get finalized results for block height %d: %w", nextHeight, err)
	}
	entries = append(entries, unfinalizedEntries...)

	return entries, nil
}

// getResultEntriesForBlock returns result entries for all ExecutionResults for a given block ID
// No errors are expected during normal operation.
func (l *ForestLoader) getResultEntriesForBlock(blockID flow.Identifier) ([]*ResultEntry, error) {
	header, err := l.headers.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get header for block %s: %w", blockID, err)
	}

	receipts, err := l.executionReceipts.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution receipts for block %s: %w", blockID, err)
	}

	entries := make([]*ResultEntry, 0)
	seen := make(map[flow.Identifier]struct{})
	for _, receipt := range receipts {
		resultID := receipt.ExecutionResult.ID()
		if _, ok := seen[resultID]; ok {
			continue // already seen this receipt
		}
		seen[resultID] = struct{}{}

		entries = append(entries, &ResultEntry{
			result: &receipt.ExecutionResult,
			header: header,
		})
	}

	return entries, nil
}

// getResultsFromFinalizedBlock returns ResultEntries for each ExecutionResult for a given finalized block
// No errors are expected during normal operation.
func (l *ForestLoader) getResultsFromFinalizedBlock(
	height uint64,
) ([]*ResultEntry, error) {
	block, err := l.blocks.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for block height %d: %w", height, err)
	}

	entries := make([]*ResultEntry, 0)
	for _, result := range block.Payload.Results {
		resultEntries, err := l.getResultEntriesForBlock(result.BlockID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, resultEntries...)
	}

	return entries, nil
}

// getResultsFromUnfinalizedBlocks returns ResultEntries for each ExecutionResult for all unfinalized blocks.
// This method recursively traverses the block tree from the given block ID, and adds all ExecutionResults
// for all descendant blocks to the entries slice.
// No errors are expected during normal operation.
func (l *ForestLoader) getResultsFromUnfinalizedBlocks(
	previousBlockID flow.Identifier,
) ([]*ResultEntry, error) {
	headers, err := l.headers.ByParentID(previousBlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get child block headers for parent block %s: %w", previousBlockID, err)
	}

	if len(headers) == 0 {
		return nil, nil
	}

	entries := make([]*ResultEntry, 0)
	for _, header := range headers {
		blockID := header.ID()
		block, err := l.blocks.ByID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not get header for block %s: %w", blockID, err)
		}

		for _, result := range block.Payload.Results {
			resultEntries, err := l.getResultEntriesForBlock(result.BlockID)
			if err != nil {
				return nil, err
			}
			entries = append(entries, resultEntries...)
		}

		childEntires, err := l.getResultsFromUnfinalizedBlocks(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not get results for child blocks of %s: %w", blockID, err)
		}

		entries = append(entries, childEntires...)
	}

	return entries, nil
}
