package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertExecutionResult inserts an execution result by ID.
func InsertExecutionResult(result *flow.ExecutionResult) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionResult, result.ID()), result)
}

// BatchInsertExecutionResult inserts an execution result by ID.
func BatchInsertExecutionResult(result *flow.ExecutionResult) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeExecutionResult, result.ID()), result)
}

// RetrieveExecutionResult retrieves a transaction by fingerprint.
func RetrieveExecutionResult(resultID flow.Identifier, result *flow.ExecutionResult) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionResult, resultID), result)
}

// IndexExecutionResult inserts an execution result ID keyed by block ID
func IndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// IndexExecutionResultByServiceEventTypeAndHeight indexes result ID by a service event type and a height it went into effect
// See handleServiceEvents documentation for in-depth explanation of what height and why it's being used.
// This index allows us to answer questions such as what is the latest EpochSetup service event up to
// block height 1000.
//
// why indexing result ID rather than service events? because the service event is included in a result,
// the indexed result id could help join the result and find the service event
//
// why using <service_event_type+sealed_height> as key rather than <sealed_height + service_event_type>?
// because having service_event_type key first allows us to scan through the index and
// filter by service event type first, and then find the result for highest height,
// see LookupLastExecutionResultForServiceEventType
func IndexExecutionResultByServiceEventTypeAndHeight(resultID flow.Identifier, eventType string, blockHeight uint64) func(*badger.Txn) error {
	typeByte, err := serviceEventTypeToPrefix(eventType)
	if err != nil {
		return func(txn *badger.Txn) error {
			return err
		}
	}
	return upsert(makePrefix(codeServiceEventIndex, typeByte, blockHeight), resultID)
}

// ReindexExecutionResult updates mapping of an execution result ID keyed by block ID
func ReindexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// BatchIndexExecutionResult inserts an execution result ID keyed by block ID into a batch
func BatchIndexExecutionResult(blockID flow.Identifier, resultID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// LookupExecutionResult finds execution result ID by block
func LookupExecutionResult(blockID flow.Identifier, resultID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexExecutionResultByBlock, blockID), resultID)
}

// RemoveExecutionResultIndex removes execution result indexed by the given blockID
func RemoveExecutionResultIndex(blockID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeIndexExecutionResultByBlock, blockID))
}

// Returns storage.ErrNotFound if no service events for the given type exist at or below the given height.
func LookupLastExecutionResultForServiceEventType(height uint64, eventType string, resultID *flow.Identifier) func(*badger.Txn) error {
	typeByte, err := serviceEventTypeToPrefix(eventType)
	if err != nil {
		return func(txn *badger.Txn) error {
			return err
		}
	}
	return findOneHighestButNoHigher(makePrefix(codeServiceEventIndex, typeByte), height, resultID)
}
