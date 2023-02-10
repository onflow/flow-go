package meter

import (
	"math"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

type MeteredStorageInteractionMap map[flow.RegisterID]uint64

type InteractionMeterParameters struct {
	storageInteractionLimit uint64
}

func DefaultInteractionMeterParameters() InteractionMeterParameters {
	return InteractionMeterParameters{
		storageInteractionLimit: math.MaxUint64,
	}
}

func (params MeterParameters) WithStorageInteractionLimit(
	maxStorageInteractionLimit uint64,
) MeterParameters {
	newParams := params
	newParams.storageInteractionLimit = maxStorageInteractionLimit
	return newParams
}

type InteractionMeter struct {
	params InteractionMeterParameters

	storageUpdateSizeMap     map[flow.RegisterID]uint64
	totalStorageBytesRead    uint64
	totalStorageBytesWritten uint64
}

func NewInteractionMeter(params InteractionMeterParameters) InteractionMeter {
	return InteractionMeter{
		params:               params,
		storageUpdateSizeMap: make(map[flow.RegisterID]uint64),
	}
}

// MeterStorageRead captures storage read bytes count and returns an error
// if it goes beyond the total interaction limit and limit is enforced
func (m *InteractionMeter) MeterStorageRead(
	storageKey flow.RegisterID,
	value flow.RegisterValue,
	enforceLimit bool,
) error {

	// all reads are on a View which only read from storage at the first read of a given key
	if _, ok := m.storageUpdateSizeMap[storageKey]; !ok {
		readByteSize := getStorageKeyValueSize(storageKey, value)
		m.totalStorageBytesRead += readByteSize
		m.storageUpdateSizeMap[storageKey] = readByteSize
	}

	return m.checkStorageInteractionLimit(enforceLimit)
}

// MeterStorageWrite captures storage written bytes count and returns an error
// if it goes beyond the total interaction limit and limit is enforced
func (m *InteractionMeter) MeterStorageWrite(
	storageKey flow.RegisterID,
	value flow.RegisterValue,
	enforceLimit bool,
) error {
	// all writes are on a View which only writes the latest updated value to storage at commit
	if old, ok := m.storageUpdateSizeMap[storageKey]; ok {
		m.totalStorageBytesWritten -= old
	}

	updateSize := getStorageKeyValueSize(storageKey, value)
	m.totalStorageBytesWritten += updateSize
	m.storageUpdateSizeMap[storageKey] = updateSize

	return m.checkStorageInteractionLimit(enforceLimit)
}

func (m *InteractionMeter) checkStorageInteractionLimit(enforceLimit bool) error {
	if enforceLimit &&
		m.TotalBytesOfStorageInteractions() > m.params.storageInteractionLimit {
		return errors.NewLedgerInteractionLimitExceededError(
			m.TotalBytesOfStorageInteractions(), m.params.storageInteractionLimit)
	}
	return nil
}

// TotalBytesReadFromStorage returns total number of byte read from storage
func (m *InteractionMeter) TotalBytesReadFromStorage() uint64 {
	return m.totalStorageBytesRead
}

// TotalBytesWrittenToStorage returns total number of byte written to storage
func (m *InteractionMeter) TotalBytesWrittenToStorage() uint64 {
	return m.totalStorageBytesWritten
}

// TotalBytesOfStorageInteractions returns total number of byte read and written from/to storage
func (m *InteractionMeter) TotalBytesOfStorageInteractions() uint64 {
	return m.TotalBytesReadFromStorage() + m.TotalBytesWrittenToStorage()
}

func getStorageKeyValueSize(
	storageKey flow.RegisterID,
	value flow.RegisterValue,
) uint64 {
	return uint64(len(storageKey.Owner) + len(storageKey.Key) + len(value))
}

func GetStorageKeyValueSizeForTesting(
	storageKey flow.RegisterID,
	value flow.RegisterValue,
) uint64 {
	return getStorageKeyValueSize(storageKey, value)
}

func (m *InteractionMeter) GetStorageUpdateSizeMapForTesting() MeteredStorageInteractionMap {
	return m.storageUpdateSizeMap
}

func (m *InteractionMeter) Merge(child InteractionMeter) {
	for key, value := range child.storageUpdateSizeMap {
		m.storageUpdateSizeMap[key] = value
	}
	m.totalStorageBytesRead += child.TotalBytesReadFromStorage()
	m.totalStorageBytesWritten += child.TotalBytesWrittenToStorage()
}
