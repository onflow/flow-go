package database

import (
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/model/flow"
)

// MeteredDatabase wrapper around the database purposely built for testing and benchmarking.
type MeteredDatabase struct {
	*Database
}

// NewMeteredDatabase create a database wrapper purposely built for testing and benchmarking.
func NewMeteredDatabase(led atree.Ledger, flowEVMRootAddress flow.Address) (*MeteredDatabase, error) {
	database, err := NewDatabase(led, flowEVMRootAddress)
	if err != nil {
		return nil, err
	}

	return &MeteredDatabase{
		Database: database,
	}, nil
}

func (m *MeteredDatabase) DropCache() {
	m.storage.DropCache()
}

func (m *MeteredDatabase) BytesRetrieved() int {
	return m.baseStorage.BytesRetrieved()
}

func (m *MeteredDatabase) BytesStored() int {
	return m.baseStorage.BytesStored()
}
func (m *MeteredDatabase) ResetReporter() {
	m.baseStorage.ResetReporter()
	m.baseStorage.Size()
	m.storage.Count()
}
