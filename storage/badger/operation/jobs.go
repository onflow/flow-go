package operation

import "github.com/dgraph-io/badger/v2"

// RetrieveProcessedIndex returns the processed index for a job consumer
func RetrieveProcessedIndex(jobName string, processed *int) func(*badger.Txn) error {
	return retrieve(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

func InsertProcessedIndex(jobName string, processed int) func(*badger.Txn) error {
	return insert(makePrefix(codeJobConsumerProcessed, jobName), processed)
}

// SetProcessedIndex updates the processed index for a job consumer with given index
func SetProcessedIndex(jobName string, processed int) func(*badger.Txn) error {
	return update(makePrefix(codeJobConsumerProcessed, jobName), processed)
}
