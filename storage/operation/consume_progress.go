package operation

import (
	"github.com/onflow/flow-go/storage"
)

// RetrieveProcessedIndex returns the processed index for a job consumer
func RetrieveProcessedIndex(r storage.Reader, jobName string, processed *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeJobConsumerProcessed, jobName), processed)
}

// SetProcessedIndex updates the processed index for a job consumer with given index
func SetProcessedIndex(w storage.Writer, jobName string, processed uint64) error {
	return UpsertByKey(w, MakePrefix(codeJobConsumerProcessed, jobName), processed)
}
