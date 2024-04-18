package snapshot

import (
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type MigrationSnapshotType int

const (
	// LargeChangeSetOrReadonlySnapshot creates a `map[flow.RegisterID]*ledger.Payload` from the payloads
	// It is more efficient when the number of payloads that are going to change is either 0
	// or large compared to the total number of payloads (more than 30% of the current number of payloads).
	LargeChangeSetOrReadonlySnapshot MigrationSnapshotType = iota
	// SmallChangeSetSnapshot creates a map of indexes `map[flow.RegisterID]int` from the payloads array
	// It is more efficient when the number of payloads that are going to change is small
	// compared to the total number of payloads (less than 30% of the current number of payloads).
	SmallChangeSetSnapshot
)

type MigrationSnapshot interface {
	snapshot.StorageSnapshot

	Exists(id flow.RegisterID) bool
	ApplyChangesAndGetNewPayloads(
		changes map[flow.RegisterID]flow.RegisterValue,
		expectedChangeAddresses map[flow.Address]struct{},
		logger zerolog.Logger,
	) ([]*ledger.Payload, error)

	Len() int
	PayloadMap() map[flow.RegisterID]*ledger.Payload
}

func NewPayloadSnapshot(
	payloads []*ledger.Payload,
	snapshotType MigrationSnapshotType,
) (MigrationSnapshot, error) {
	switch snapshotType {
	case LargeChangeSetOrReadonlySnapshot:
		return newMapSnapshot(payloads)
	case SmallChangeSetSnapshot:
		return newIndexMapSnapshot(payloads)
	default:
		// should never happen
		panic("unknown snapshot type")
	}

}

type mapSnapshot struct {
	Payloads map[flow.RegisterID]*ledger.Payload
}

var _ MigrationSnapshot = (*mapSnapshot)(nil)

func newMapSnapshot(payloads []*ledger.Payload) (*mapSnapshot, error) {
	l := &mapSnapshot{
		Payloads: make(map[flow.RegisterID]*ledger.Payload, len(payloads)),
	}
	for _, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		id, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}
		l.Payloads[id] = payload
	}
	return l, nil
}

func (p *mapSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := p.Payloads[id]
	if !exists {
		return nil, nil
	}
	return value.Value(), nil
}

func (p *mapSnapshot) Exists(id flow.RegisterID) bool {
	_, exists := p.Payloads[id]
	return exists
}

func (p *mapSnapshot) Len() int {
	return len(p.Payloads)
}

func (p *mapSnapshot) PayloadMap() map[flow.RegisterID]*ledger.Payload {
	return p.Payloads
}

// ApplyChangesAndGetNewPayloads applies the given changes to the snapshot and returns the new payloads.
// the snapshot is destroyed.
func (p *mapSnapshot) ApplyChangesAndGetNewPayloads(
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) ([]*ledger.Payload, error) {
	originalPayloads := p.Payloads

	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// Add all new payloads.
	for id, value := range changes {
		delete(originalPayloads, id)
		if len(value) == 0 {
			continue
		}

		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", id.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedChangeAddresses).
					Hex("value", value).
					Msg("key is part of the change set, but is for a different account")
			}
		}

		key := convert.RegisterIDToLedgerKey(id)
		newPayloads = append(newPayloads, ledger.NewPayload(key, value))
	}

	// Add any old payload that wasn't updated.
	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// This is strange, but we don't want to add empty values. Log it.
			logger.Warn().Msgf("empty value for key %s", id)
			continue
		}

		newPayloads = append(newPayloads, value)
	}

	return newPayloads, nil
}

type indexMapSnapshot struct {
	reverseMap map[flow.RegisterID]int
	payloads   []*ledger.Payload
}

var _ MigrationSnapshot = (*indexMapSnapshot)(nil)

func newIndexMapSnapshot(payloads []*ledger.Payload) (*indexMapSnapshot, error) {
	payloadsCopy := make([]*ledger.Payload, len(payloads))
	copy(payloadsCopy, payloads)
	l := &indexMapSnapshot{
		reverseMap: make(map[flow.RegisterID]int, len(payloads)),
		payloads:   payloadsCopy,
	}
	for i, payload := range payloadsCopy {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		id, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}
		l.reverseMap[id] = i
	}
	return l, nil
}

func (p *indexMapSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	index, exists := p.reverseMap[id]
	if !exists {
		return nil, nil
	}
	return p.payloads[index].Value(), nil
}

func (p *indexMapSnapshot) Exists(id flow.RegisterID) bool {
	_, exists := p.reverseMap[id]
	return exists
}

func (p *indexMapSnapshot) Len() int {
	return len(p.payloads)
}

func (p *indexMapSnapshot) PayloadMap() map[flow.RegisterID]*ledger.Payload {
	result := make(map[flow.RegisterID]*ledger.Payload, len(p.payloads))
	for id, index := range p.reverseMap {
		result[id] = p.payloads[index]
	}
	return result
}

// ApplyChangesAndGetNewPayloads applies the given changes to the snapshot and returns the new payloads.
// the snapshot is destroyed.
func (p *indexMapSnapshot) ApplyChangesAndGetNewPayloads(
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) ([]*ledger.Payload, error) {

	// append all new payloads at once at the end
	newPayloads := make([]*ledger.Payload, 0, len(changes))
	deletedPayloads := make([]int, 0, len(changes))

	for id, value := range changes {
		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", id.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedChangeAddresses).
					Hex("value", value).
					Msg("key is part of the change set, but is for a different account")
			}
		}

		existingItemIndex, exists := p.reverseMap[id]
		if !exists {
			if len(value) == 0 {
				// do not add new empty values
				continue
			}
			newPayloads = append(newPayloads, ledger.NewPayload(convert.RegisterIDToLedgerKey(id), value))
		} else {
			if len(value) == 0 {
				// do not add new empty values
				deletedPayloads = append(deletedPayloads, existingItemIndex)
				continue
			}

			// update existing payload
			p.payloads[existingItemIndex] = ledger.NewPayload(convert.RegisterIDToLedgerKey(id), value)
		}
	}

	// remove deleted payloads by moving the last ones to the deleted positions
	// and then re-slicing the array
	sort.Ints(deletedPayloads)
	for i := len(deletedPayloads) - 1; i >= 0; i-- {
		index := deletedPayloads[i]
		// take items from the very end of the array
		p.payloads[index] = p.payloads[len(p.payloads)-1-(len(deletedPayloads)-1-i)]
	}
	p.payloads = p.payloads[:len(p.payloads)-len(deletedPayloads)]

	result := append(p.payloads, newPayloads...)

	// destroy the snapshot to prevent further use
	p.payloads = nil
	p.reverseMap = nil

	return result, nil
}
