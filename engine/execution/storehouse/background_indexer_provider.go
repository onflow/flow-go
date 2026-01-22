package storehouse

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDataRegisterUpdatesProvider struct {
	dataStore execution_data.ExecutionDataGetter
	results   storage.ExecutionResultsReader
}

var _ RegisterUpdatesProvider = (*ExecutionDataRegisterUpdatesProvider)(nil)

func NewExecutionDataRegisterUpdatesProvider(
	dataStore execution_data.ExecutionDataGetter,
	results storage.ExecutionResultsReader,
) *ExecutionDataRegisterUpdatesProvider {
	return &ExecutionDataRegisterUpdatesProvider{
		dataStore: dataStore,
		results:   results,
	}
}

func (p *ExecutionDataRegisterUpdatesProvider) RegisterUpdatesByBlockID(ctx context.Context, blockID flow.Identifier) (flow.RegisterEntries, bool, error) {
	result, err := p.results.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// No execution result for this block
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get execution result for block %v: %w", blockID, err)
	}

	data, err := p.dataStore.Get(ctx, result.ExecutionDataID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get execution data %v: %w", result.ExecutionDataID, err)
	}

	// Collect register updates from all chunks
	// Use a map to track the last update for each path (since there can be multiple
	// updates to the same path within a block, we only persist the last one)
	registerUpdates := make(map[ledger.Path]*ledger.Payload)

	for _, chunk := range data.ChunkExecutionDatas {
		// Collect register updates from this chunk
		if chunk.TrieUpdate == nil {
			continue
		}

		// Sanity check: there must be a one-to-one mapping between paths and payloads
		if len(chunk.TrieUpdate.Paths) != len(chunk.TrieUpdate.Payloads) {
			return nil, false, fmt.Errorf("number of ledger paths (%d) does not match number of ledger payloads (%d)",
				len(chunk.TrieUpdate.Paths), len(chunk.TrieUpdate.Payloads))
		}

		// Collect registers (last update for a path within the block is persisted)
		for i, path := range chunk.TrieUpdate.Paths {
			registerUpdates[path] = chunk.TrieUpdate.Payloads[i]
		}
	}

	// Convert final payloads to register entries
	registerEntries := make(flow.RegisterEntries, 0, len(registerUpdates))
	for path, payload := range registerUpdates {
		key, value, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, false, fmt.Errorf("failed to convert payload to register entry (path: %s): %w", path.String(), err)
		}

		registerEntries = append(registerEntries, flow.RegisterEntry{
			Key:   key,
			Value: value,
		})
	}

	return registerEntries, true, nil
}
