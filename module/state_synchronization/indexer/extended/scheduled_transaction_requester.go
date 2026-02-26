package extended

import (
	"context"
	"fmt"
	"slices"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const maxLookupBatchSize = 50

// scriptExecutor is the subset of module/execution.ScriptExecutor used by ScheduledTransactionRequester.
// Defined locally to avoid an import cycle with module/execution.
type scriptExecutor interface {
	ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64) ([]byte, error)
}

// ScheduledTransactionRequester fetches scheduled transaction data from on-chain state
// by executing Cadence scripts against the FlowTransactionScheduler contract.
//
// Not safe for concurrent use.
type ScheduledTransactionRequester struct {
	executor scriptExecutor
	script   []byte
}

// NewScheduledTransactionRequester creates a new ScheduledTransactionRequester.
func NewScheduledTransactionRequester(executor scriptExecutor, chainID flow.ChainID) *ScheduledTransactionRequester {
	return &ScheduledTransactionRequester{
		executor: executor,
		script:   GetTransactionDataScript(chainID),
	}
}

// Fetch fetches scheduled transaction data for the given IDs from on-chain state at lookupHeight,
// and applies the status updates from the collected block data.
//
// No error returns are expected during normal operation.
func (r *ScheduledTransactionRequester) Fetch(
	ctx context.Context,
	lookupIDs []uint64,
	lookupHeight uint64,
	data *scheduledTransactionData,
) ([]access.ScheduledTransaction, error) {
	missingTxs, err := r.fetchMissingTxs(ctx, lookupIDs, lookupHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch missing scheduled transactions: %w", err)
	}

	updatedTxs := make([]access.ScheduledTransaction, 0, len(missingTxs))
	for _, entry := range data.executedEntries {
		if missing, ok := missingTxs[entry.event.ID]; ok {
			// set IsPlaceholder = true to signal that some information is missing because we don't know the original transaction.
			missing.IsPlaceholder = true
			missing.Status = access.ScheduledTxStatusExecuted
			missing.ExecutedTransactionID = entry.transactionID
			updatedTxs = append(updatedTxs, missing)
		}
	}
	for _, entry := range data.canceledEntries {
		if missing, ok := missingTxs[entry.event.ID]; ok {
			// set IsPlaceholder = true to signal that some information is missing because we don't know the original transaction.
			missing.IsPlaceholder = true
			missing.Status = access.ScheduledTxStatusCancelled
			missing.CancelledTransactionID = entry.transactionID
			missing.FeesReturned = uint64(entry.event.FeesReturned)
			missing.FeesDeducted = uint64(entry.event.FeesDeducted)
			updatedTxs = append(updatedTxs, missing)
		}
	}
	for _, entry := range data.failedEntries {
		if missing, ok := missingTxs[entry.scheduledTxID]; ok {
			// set IsPlaceholder = true to signal that some information is missing because we don't know the original transaction.
			missing.IsPlaceholder = true
			missing.Status = access.ScheduledTxStatusFailed
			missing.ExecutedTransactionID = entry.transactionID
			updatedTxs = append(updatedTxs, missing)
		}
	}

	if len(updatedTxs) != len(missingTxs) {
		return nil, fmt.Errorf("expected %d updated scheduled transactions, got %d", len(missingTxs), len(updatedTxs))
	}

	return updatedTxs, nil
}

func (r *ScheduledTransactionRequester) fetchMissingTxs(
	ctx context.Context,
	lookupIDs []uint64,
	height uint64,
) (map[uint64]access.ScheduledTransaction, error) {
	missingTxs := make(map[uint64]access.ScheduledTransaction, len(lookupIDs))

	for batch := range slices.Chunk(lookupIDs, maxLookupBatchSize) {
		idsArg, err := EncodeGetTransactionDataArg(batch)
		if err != nil {
			return nil, fmt.Errorf("failed to build arguments: %w", err)
		}

		response, err := r.executor.ExecuteAtBlockHeight(ctx, r.script, [][]byte{idsArg}, height)
		if err != nil {
			return nil, fmt.Errorf("failed to execute at block height: %w", err)
		}

		results, err := jsoncdc.Decode(nil, response)
		if err != nil {
			return nil, fmt.Errorf("failed to decode scheduled transactions: %w", err)
		}

		array, ok := results.(cadence.Array)
		if !ok {
			return nil, fmt.Errorf("expected Array result, got %T", results)
		}

		for i, result := range array.Values {
			opt, ok := result.(cadence.Optional)
			if !ok {
				return nil, fmt.Errorf("expected Optional at index %d, got %T", i, result)
			}
			if opt.Value == nil {
				return nil, fmt.Errorf("scheduled transaction %d had event, but is not found on-chain", batch[i])
			}
			decoded, err := decodeTransactionData(opt.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to decode scheduled transaction %d: %w", batch[i], err)
			}
			missingTxs[decoded.ID] = decoded
		}
	}

	return missingTxs, nil
}

// GetTransactionDataScript returns the Cadence script used for JIT scheduled transaction
// lookups on the given chain. Exposed for testing.
func GetTransactionDataScript(chainID flow.ChainID) []byte {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return []byte(fmt.Sprintf(getTransactionDataScriptTemplate, sc.FlowTransactionScheduler.Address.Hex()))
}
