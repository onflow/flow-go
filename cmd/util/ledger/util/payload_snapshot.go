package util

import (
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type MigrationStorageSnapshot interface {
	snapshot.StorageSnapshot

	Exists(id flow.RegisterID) bool
	ApplyChangesAndGetNewPayloads(
		changes map[flow.RegisterID]flow.RegisterValue,
		expectedChangeAddresses map[flow.Address]struct{},
	) ([]*ledger.Payload, error)

	Len() int
	PayloadMap() map[flow.RegisterID]*ledger.Payload
}

type PayloadSnapshot struct {
	Payloads map[flow.RegisterID]*ledger.Payload
	log      zerolog.Logger
}

var _ MigrationStorageSnapshot = PayloadSnapshot{}

func NewPayloadSnapshot(log zerolog.Logger, payloads []*ledger.Payload) (*PayloadSnapshot, error) {
	l := &PayloadSnapshot{
		Payloads: make(map[flow.RegisterID]*ledger.Payload, len(payloads)),
		log:      log,
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

func (p PayloadSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	value, exists := p.Payloads[id]
	if !exists {
		return nil, nil
	}
	return value.Value(), nil
}

func (p PayloadSnapshot) Exists(id flow.RegisterID) bool {
	_, exists := p.Payloads[id]
	return exists
}

func (p PayloadSnapshot) Len() int {
	return len(p.Payloads)
}

func (p PayloadSnapshot) PayloadMap() map[flow.RegisterID]*ledger.Payload {
	return p.Payloads
}

// ApplyChangesAndGetNewPayloads applies the given changes to the snapshot and returns the new payloads.
// the snapshot is destroyed.
func (p PayloadSnapshot) ApplyChangesAndGetNewPayloads(
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
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
				p.log.Error().
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
			p.log.Warn().Msgf("empty value for key %s", id)
			continue
		}

		newPayloads = append(newPayloads, value)
	}

	return newPayloads, nil
}

type MapBasedPayloadSnapshot struct {
	reverseMap map[flow.RegisterID]int
	payloads   []*ledger.Payload
	log        zerolog.Logger
}

var _ MigrationStorageSnapshot = (*MapBasedPayloadSnapshot)(nil)

func NewMapBasedPayloadSnapshot(log zerolog.Logger, payloads []*ledger.Payload) (*MapBasedPayloadSnapshot, error) {
	return NewMapBasedPayloadSnapshotWithWorkers(log, payloads, 1)
}

// NewMapBasedPayloadSnapshotWithWorkers creates a new MapBasedPayloadSnapshot with the given number of workers.
// The workers are used to create the reverse map in parallel.
func NewMapBasedPayloadSnapshotWithWorkers(log zerolog.Logger, payloads []*ledger.Payload, workers int) (*MapBasedPayloadSnapshot, error) {

	// limit the number of workers to prevent too many small jobs
	const minPayloadsPerWorker = 10000
	if len(payloads)/minPayloadsPerWorker < workers {
		workers = len(payloads) / minPayloadsPerWorker
	}
	if workers < 1 {
		workers = 1
	}

	// log the creation of the snapshot if it is large
	if len(payloads) > minPayloadsPerWorker {
		start := time.Now()
		log.Info().
			Int("payloads", len(payloads)).
			Int("workers", workers).
			Msgf("Creating payload snapshot")

		defer func() {
			log.Info().
				Int("payloads", len(payloads)).
				Int("workers", workers).
				Dur("duration", time.Since(start)).
				Msgf("Created payload snapshot")
		}()
	}

	payloadsCopy := make([]*ledger.Payload, len(payloads))
	copy(payloadsCopy, payloads)

	type result struct {
		minimap map[flow.RegisterID]int
		err     error
	}

	minimaps := make(chan result, workers)
	defer close(minimaps)

	// create the minimaps in parallel
	for i := 0; i < workers; i++ {
		start := i * len(payloadsCopy) / workers
		end := (i + 1) * len(payloadsCopy) / workers
		if end > len(payloadsCopy) {
			end = len(payloadsCopy)
		}
		go func(payloads []*ledger.Payload) {
			minimap := make(map[flow.RegisterID]int, len(payloads))
			for i, payload := range payloads {
				key, err := payload.Key()
				if err != nil {
					minimaps <- result{nil, err}
					return
				}
				id, err := convert.LedgerKeyToRegisterID(key)
				if err != nil {
					minimaps <- result{nil, err}
					return
				}
				minimap[id] = i
			}
			minimaps <- result{minimap, nil}
		}(payloadsCopy[start:end])
	}

	// merge the minimaps in parallel
	pairedMinimaps := make(
		chan struct {
			left  result
			right result
		}, workers/2)

	go func() {
		// we have to pair the minimaps to merge them
		// we have to merge a total of workers-1 times
		// before we are left with only one map
		numberOfPairings := workers - 1
		for i := 0; i < numberOfPairings; i++ {
			left := <-minimaps
			right := <-minimaps
			pairedMinimaps <- struct {
				left  result
				right result
			}{left, right}
		}
		close(pairedMinimaps)
	}()

	// only half of the workers are needed to merge the maps
	wg := sync.WaitGroup{}
	wg.Add(workers / 2)
	for i := 0; i < workers/2; i++ {
		go func() {
			defer wg.Done()
			for pair := range pairedMinimaps {
				if pair.left.err != nil {
					minimaps <- result{nil, pair.left.err}
					return
				}
				if pair.right.err != nil {
					minimaps <- result{nil, pair.right.err}
					return
				}
				// merge the two minimaps
				maps.Copy(pair.left.minimap, pair.right.minimap)
				minimaps <- result{pair.left.minimap, nil}
			}
		}()
	}

	// wait for all workers to finish
	wg.Wait()

	r := <-minimaps
	if r.err != nil {
		return nil, r.err
	}

	l := &MapBasedPayloadSnapshot{
		reverseMap: r.minimap,
		payloads:   payloadsCopy,
		log:        log,
	}
	return l, nil
}

func (p *MapBasedPayloadSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	index, exists := p.reverseMap[id]
	if !exists {
		return nil, nil
	}
	return p.payloads[index].Value(), nil
}

func (p *MapBasedPayloadSnapshot) Exists(id flow.RegisterID) bool {
	_, exists := p.reverseMap[id]
	return exists
}

func (p *MapBasedPayloadSnapshot) Len() int {
	return len(p.payloads)
}

func (p *MapBasedPayloadSnapshot) PayloadMap() map[flow.RegisterID]*ledger.Payload {
	result := make(map[flow.RegisterID]*ledger.Payload, len(p.payloads))
	for id, index := range p.reverseMap {
		result[id] = p.payloads[index]
	}
	return result
}

// ApplyChangesAndGetNewPayloads applies the given changes to the snapshot and returns the new payloads.
// the snapshot is destroyed.
func (p *MapBasedPayloadSnapshot) ApplyChangesAndGetNewPayloads(
	changes map[flow.RegisterID]flow.RegisterValue,
	expectedChangeAddresses map[flow.Address]struct{},
) ([]*ledger.Payload, error) {

	// append all new payloads at once at the end
	newPayloads := make([]*ledger.Payload, 0, len(changes))
	deletedPayloads := make([]int, 0, len(changes))

	for id, value := range changes {
		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				p.log.Error().
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
