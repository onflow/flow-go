package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func RetrieveJobLatestIndex(queue string, index *uint64) func(storage.Reader) error {
	return retrieveR(makePrefix(codeJobQueuePointer, queue), index)
}

func InitJobLatestIndex(queue string, index uint64) func(storage.Writer) error {
	return insertW(makePrefix(codeJobQueuePointer, queue), index)
}

func SetJobLatestIndex(queue string, index uint64) func(storage.Writer) error {
	return insertW(makePrefix(codeJobQueuePointer, queue), index)
}

// RetrieveJobAtIndex returns the entity at the given index
func RetrieveJobAtIndex(queue string, index uint64, entity *flow.Identifier) func(storage.Reader) error {
	return retrieveR(makePrefix(codeJobQueue, queue, index), entity)
}

// InsertJobAtIndex insert an entity ID at the given index
func InsertJobAtIndex(queue string, index uint64, entity flow.Identifier) func(storage.Writer) error {
	return insertW(makePrefix(codeJobQueue, queue, index), entity)
}

// RetrieveProcessedIndex returns the processed index for a job consumer
func RetrieveProcessedIndex(jobName string, processed *uint64) func(storage.Reader) error {
	return retrieveR(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

// SetProcessedIndex updates the processed index for a job consumer with given index
func SetProcessedIndex(jobName string, processed uint64) func(storage.Writer) error {
	return insertW(makePrefix(codeJobConsumerProcessed, jobName), processed)
}
