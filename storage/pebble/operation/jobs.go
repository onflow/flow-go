package operation

import (
	"github.com/cockroachdb/pebble"
)

// RetrieveProcessedIndex returns the processed index for a job consumer
func RetrieveProcessedIndex(jobName string, processed *uint64) func(pebble.Reader) error {
	return retrieve(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

func InsertProcessedIndex(jobName string, processed uint64) func(pebble.Writer) error {
	return insert(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

// SetProcessedIndex updates the processed index for a job consumer with given index
func SetProcessedIndex(jobName string, processed uint64) func(pebble.Writer) error {
	return insert(makePrefix(codeJobConsumerProcessed, jobName), processed)
}
