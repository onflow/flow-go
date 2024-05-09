package snapshot

import (
	"github.com/onflow/flow-go/model/flow"
)

// Note: StorageSnapshot must be thread safe (or immutable).
type StorageSnapshot interface {
	// Get returns the register id's value, or an empty RegisterValue if the id
	// is not found.  Get should be idempotent (i.e., the same value is returned
	// for the same id).
	Get(id flow.RegisterID) (flow.RegisterValue, error)
}

type EmptyStorageSnapshot struct{}

func (EmptyStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return nil, nil
}

type ReadFuncStorageSnapshot struct {
	ReadFunc func(flow.RegisterID) (flow.RegisterValue, error)
}

func NewReadFuncStorageSnapshot(
	readFunc func(flow.RegisterID) (flow.RegisterValue, error),
) StorageSnapshot {
	return &ReadFuncStorageSnapshot{
		ReadFunc: readFunc,
	}
}

func (storage ReadFuncStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return storage.ReadFunc(id)
}

type Peeker interface {
	Peek(id flow.RegisterID) (flow.RegisterValue, error)
}

func NewPeekerStorageSnapshot(peeker Peeker) StorageSnapshot {
	return NewReadFuncStorageSnapshot(peeker.Peek)
}

type MapStorageSnapshot map[flow.RegisterID]flow.RegisterValue

func (storage MapStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return storage[id], nil
}
