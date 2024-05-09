package meter

import (
	"math"

	"github.com/rs/zerolog/log"

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

// InteractionMeter is a meter that tracks storage interaction
// Only the first read of a given key is counted
// Only the last write of a given key is counted
type InteractionMeter struct {
	params InteractionMeterParameters

	reads  map[flow.RegisterID]uint64
	writes map[flow.RegisterID]uint64

	totalStorageBytesRead    uint64
	totalStorageBytesWritten uint64
}

func NewInteractionMeter(params InteractionMeterParameters) InteractionMeter {
	return InteractionMeter{
		params: params,
		reads:  make(map[flow.RegisterID]uint64),
		writes: make(map[flow.RegisterID]uint64),
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
	if _, ok := m.reads[storageKey]; !ok {
		readByteSize := getStorageKeyValueSize(storageKey, value)
		m.totalStorageBytesRead += readByteSize
		m.reads[storageKey] = readByteSize
	}

	return m.checkStorageInteractionLimit(enforceLimit)
}

// MeterStorageWrite captures storage written bytes count and returns an error
// if it goes beyond the total interaction limit and limit is enforced.
// If a key is written multiple times, only the last write is counted.
// If a key is written before it has been read, next time it will be read it will be from the view,
// not from storage, so count it as read 0.
func (m *InteractionMeter) MeterStorageWrite(
	storageKey flow.RegisterID,
	value flow.RegisterValue,
	enforceLimit bool,
) error {
	updateSize := getStorageKeyValueSize(storageKey, value)
	m.replaceWrite(storageKey, updateSize)

	if _, ok := m.reads[storageKey]; !ok {
		// write without read, count as read 0 because next time you read it the written value
		// will be returned from cache, so no interaction with storage will happen.
		m.reads[storageKey] = 0
	}

	return m.checkStorageInteractionLimit(enforceLimit)
}

// replaceWrite replaces the write size of a given key with the new size, because
// only the last write of a given key is counted towards the total interaction limit.
// These are the only write usages of `m.totalStorageBytesWritten` and `m.writes`,
// which means that `m.totalStorageBytesWritten` can never become negative.
// oldSize is always <= m.totalStorageBytesWritten.
func (m *InteractionMeter) replaceWrite(
	k flow.RegisterID,
	newSize uint64,
) {
	totalBefore := m.totalStorageBytesWritten

	// remove old write
	oldSize := m.writes[k]
	m.totalStorageBytesWritten -= oldSize

	// sanity check
	// this should never happen, but if it does, it should be fatal
	if m.totalStorageBytesWritten > totalBefore {
		log.Fatal().
			Str("component", "interaction_meter").
			Uint64("total", totalBefore).
			Uint64("subtract", oldSize).
			Msg("totalStorageBytesWritten would have become negative")
	}

	// add new write
	m.writes[k] = newSize
	m.totalStorageBytesWritten += newSize
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

func (m *InteractionMeter) GetStorageRWSizeMapForTesting() (
	reads MeteredStorageInteractionMap,
	writes MeteredStorageInteractionMap,
) {
	return m.reads, m.writes
}

// Merge merges the child interaction meter into the parent interaction meter
// Prioritise parent reads because they happened first
// Prioritise child writes because they happened last
func (m *InteractionMeter) Merge(child InteractionMeter) {
	for key, value := range child.reads {
		_, parentRead := m.reads[key]
		if parentRead {
			// avoid metering the same read more than once, because a second read
			// is from the cache
			continue
		}

		m.reads[key] = value
		m.totalStorageBytesRead += value
	}

	for key, value := range child.writes {
		m.replaceWrite(key, value)
	}
}
