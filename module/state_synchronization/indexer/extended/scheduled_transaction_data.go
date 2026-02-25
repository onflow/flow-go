package extended

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
)

// getTransactionDataScriptTemplate is a Cadence script template for batch-fetching
// FlowTransactionScheduler.TransactionData by ID. The %s placeholder is replaced with
// the FlowTransactionScheduler contract address hex string.
//
// The script accepts a single [UInt64] argument (the scheduled transaction IDs to fetch)
// and returns [FlowTransactionScheduler.TransactionData?], where each element corresponds
// to the input ID in order. Elements are nil for IDs that do not exist.
const getTransactionDataScriptTemplate = `
import FlowTransactionScheduler from 0x%s

/// Returns the TransactionData for each of the given scheduled transaction IDs.
/// Returns nil for any ID that does not exist.
access(all) fun main(ids: [UInt64]): [FlowTransactionScheduler.TransactionData?] {
    let results: [FlowTransactionScheduler.TransactionData?] = []
    for id in ids {
        results.append(FlowTransactionScheduler.getTransactionData(id: id))
    }
    return results
}
`

// EncodeGetTransactionDataArg encodes a slice of scheduled transaction IDs as a
// JSON-CDC [UInt64] array suitable for passing as the script argument when executing
// a script generated from [getTransactionDataScriptTemplate].
//
// No error returns are expected during normal operation.
func EncodeGetTransactionDataArg(ids []uint64) ([]byte, error) {
	values := make([]cadence.Value, len(ids))
	for i, id := range ids {
		values[i] = cadence.UInt64(id)
	}
	encoded, err := jsoncdc.Encode(cadence.NewArray(values))
	if err != nil {
		return nil, fmt.Errorf("failed to JSON-CDC encode IDs array: %w", err)
	}
	return encoded, nil
}

// DecodeTransactionDataResults decodes the JSON-CDC response from a batch
// GetTransactionData script execution. The ids slice must match the order of IDs
// passed when the script was called.
//
// Returns a map from scheduled transaction ID to decoded [access.ScheduledTransaction].
// IDs for which the contract returned nil (not found on-chain) are omitted from the map.
// The returned entries have [access.ScheduledTxStatusScheduled] status, since
// TransactionData reflects the initially scheduled state.
//
// Any error indicates that the response is malformed.
func DecodeTransactionDataResults(response []byte, ids []uint64) (map[uint64]*access.ScheduledTransaction, error) {
	value, err := jsoncdc.Decode(nil, response)
	if err != nil {
		return nil, fmt.Errorf("failed to JSON-CDC decode script result: %w", err)
	}

	array, ok := value.(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("expected Array result, got %T", value)
	}

	if len(array.Values) != len(ids) {
		return nil, fmt.Errorf("expected %d results, got %d", len(ids), len(array.Values))
	}

	results := make(map[uint64]*access.ScheduledTransaction, len(ids))
	for i, elem := range array.Values {
		opt, ok := elem.(cadence.Optional)
		if !ok {
			return nil, fmt.Errorf("expected Optional at index %d, got %T", i, elem)
		}
		if opt.Value == nil {
			continue
		}

		tx, err := decodeTransactionData(opt.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to decode TransactionData at index %d (id=%d): %w", i, ids[i], err)
		}
		results[ids[i]] = &tx
	}

	return results, nil
}

// decodeTransactionData decodes a Cadence FlowTransactionScheduler.TransactionData
// struct value into an [access.ScheduledTransaction].
//
// Any error indicates that the value is malformed.
func decodeTransactionData(value cadence.Value) (access.ScheduledTransaction, error) {
	type transactionDataRaw struct {
		ID                               uint64           `cadence:"id"`
		Priority                         uint8            `cadence:"priority"`
		Timestamp                        cadence.UFix64   `cadence:"timestamp"`
		ExecutionEffort                  uint64           `cadence:"executionEffort"`
		Fees                             cadence.UFix64   `cadence:"fees"`
		TransactionHandlerOwner          cadence.Address  `cadence:"transactionHandlerOwner"`
		TransactionHandlerTypeIdentifier string           `cadence:"transactionHandlerTypeIdentifier"`
		TransactionHandlerUUID           uint64           `cadence:"transactionHandlerUUID"`
		TransactionHandlerPublicPath     cadence.Optional `cadence:"transactionHandlerPublicPath"`
	}

	composite, ok := value.(cadence.Composite)
	if !ok {
		return access.ScheduledTransaction{}, fmt.Errorf("expected Composite value, got %T", value)
	}

	var raw transactionDataRaw
	if err := cadence.DecodeFields(composite, &raw); err != nil {
		return access.ScheduledTransaction{}, fmt.Errorf("failed to decode TransactionData fields: %w", err)
	}

	publicPath, err := events.PathFromOptional(raw.TransactionHandlerPublicPath)
	if err != nil {
		return access.ScheduledTransaction{}, fmt.Errorf("failed to decode 'transactionHandlerPublicPath' field: %w", err)
	}

	return access.ScheduledTransaction{
		ID:                               raw.ID,
		Priority:                         access.ScheduledTransactionPriority(raw.Priority),
		Timestamp:                        uint64(raw.Timestamp),
		ExecutionEffort:                  raw.ExecutionEffort,
		Fees:                             uint64(raw.Fees),
		TransactionHandlerOwner:          flow.Address(raw.TransactionHandlerOwner),
		TransactionHandlerTypeIdentifier: raw.TransactionHandlerTypeIdentifier,
		TransactionHandlerUUID:           raw.TransactionHandlerUUID,
		TransactionHandlerPublicPath:     publicPath,
		Status:                           access.ScheduledTxStatusScheduled,
	}, nil
}
