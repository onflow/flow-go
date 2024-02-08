package handler

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

// WrappedBackend wraps the backend to flag
// if a returned error from the lower levels was
// due to an backend error
type WrappedBackend struct {
	backend types.Backend
}

var _ types.Backend = &WrappedBackend{}

func (wb *WrappedBackend) GetValue(owner, key []byte) ([]byte, error) {
	val, err := wb.backend.GetValue(owner, key)
	if err != nil {
		return nil, NewBackendError(err)
	}
	return val, nil
}

func (wb *WrappedBackend) SetValue(owner, key, value []byte) error {
	err := wb.backend.SetValue(owner, key, value)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (wb *WrappedBackend) ValueExists(owner, key []byte) (bool, error) {
	b, err := wb.backend.ValueExists(owner, key)
	if err != nil {
		return false, NewBackendError(err)
	}
	return b, nil
}

func (wb *WrappedBackend) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	index, err := wb.backend.AllocateStorageIndex(owner)
	if err != nil {
		return atree.StorageIndex{}, NewBackendError(err)
	}
	return index, nil
}

func (wb *WrappedBackend) MeterComputation(kind common.ComputationKind, intensity uint) error {
	err := wb.backend.MeterComputation(kind, intensity)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (wb *WrappedBackend) ComputationUsed() (uint64, error) {
	val, err := wb.backend.ComputationUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (wb *WrappedBackend) ComputationIntensities() meter.MeteredComputationIntensities {
	return wb.backend.ComputationIntensities()
}

func (wb *WrappedBackend) ComputationAvailable(kind common.ComputationKind, intensity uint) bool {
	return wb.backend.ComputationAvailable(kind, intensity)
}

func (wb *WrappedBackend) MeterMemory(usage common.MemoryUsage) error {
	err := wb.backend.MeterMemory(usage)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (wb *WrappedBackend) MemoryUsed() (uint64, error) {
	val, err := wb.backend.MemoryUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (wb *WrappedBackend) MeterEmittedEvent(byteSize uint64) error {
	err := wb.backend.MeterEmittedEvent(byteSize)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (wb *WrappedBackend) TotalEmittedEventBytes() uint64 {
	return wb.backend.TotalEmittedEventBytes()
}

func (wb *WrappedBackend) InteractionUsed() (uint64, error) {
	val, err := wb.backend.InteractionUsed()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (wb *WrappedBackend) EmitEvent(event cadence.Event) error {
	err := wb.backend.EmitEvent(event)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}

func (wb *WrappedBackend) Events() flow.EventsList {
	return wb.backend.Events()

}

func (wb *WrappedBackend) ServiceEvents() flow.EventsList {
	return wb.backend.ServiceEvents()
}

func (wb *WrappedBackend) ConvertedServiceEvents() flow.ServiceEventList {
	return wb.backend.ConvertedServiceEvents()
}

func (wb *WrappedBackend) Reset() {
	wb.backend.Reset()
}

func (wb *WrappedBackend) GetCurrentBlockHeight() (uint64, error) {
	val, err := wb.backend.GetCurrentBlockHeight()
	if err != nil {
		return 0, NewBackendError(err)
	}
	return val, nil
}

func (wb *WrappedBackend) GetBlockAtHeight(height uint64) (
	runtime.Block,
	bool,
	error,
) {
	val, found, err := wb.backend.GetBlockAtHeight(height)
	if err != nil {
		return runtime.Block{}, false, NewBackendError(err)
	}
	return val, found, nil
}

func (wb *WrappedBackend) ReadRandom(buffer []byte) error {
	err := wb.backend.ReadRandom(buffer)
	if err != nil {
		return NewBackendError(err)
	}
	return nil
}
