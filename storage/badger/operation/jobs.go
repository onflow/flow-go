package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func RetrieveJobLatestIndex(queue string, index *int64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeJobQueuePointer, queue), index)
}

func InitJobLatestIndex(queue string, index int64) func(*badger.Txn) error {
	return insert(makePrefix(codeJobQueuePointer, queue), index)
}

func SetJobLatestIndex(queue string, index int64) func(*badger.Txn) error {
	return update(makePrefix(codeJobQueuePointer, queue), index)
}

// RetrieveJobAtIndex returns the entity at the given index
func RetrieveJobAtIndex(queue string, index int64, entity *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeJobQueue, queue, uint64(index)), entity)
}

// InsertJobAtIndex insert an entity ID at the given index
func InsertJobAtIndex(queue string, index int64, entity flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeJobQueue, queue, uint64(index)), entity)
}

// RetrieveProcessedIndex returns the processed index for a job consumer
func RetrieveProcessedIndex(jobName string, processed *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

func InsertProcessedIndex(jobName string, processed uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

// SetProcessedIndex updates the processed index for a job consumer with given index
func SetProcessedIndex(jobName string, processed uint64) func(*badger.Txn) error {
	return update(makePrefix(codeJobConsumerProcessed, jobName), processed)
}
