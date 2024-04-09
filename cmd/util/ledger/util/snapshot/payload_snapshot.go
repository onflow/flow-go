package snapshot

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
	log zerolog.Logger,
	payloads []*ledger.Payload,
	snapshotType MigrationSnapshotType,
	nWorkers int,
) (MigrationSnapshot, error) {
	switch snapshotType {
	case LargeChangeSetOrReadonlySnapshot:
		return newMapSnapshot(log, payloads, nWorkers)
	case SmallChangeSetSnapshot:
		return newIndexMapSnapshot(log, payloads, nWorkers)
	default:
		// should never happen
		panic("unknown snapshot type")
	}
}

type mapSnapshot struct {
	Payloads map[flow.RegisterID]*ledger.Payload
}

var _ MigrationSnapshot = (*mapSnapshot)(nil)

func newMapSnapshot(log zerolog.Logger, payloads []*ledger.Payload, workers int) (*mapSnapshot, error) {
	pm, err := createLargeMap(log, payloads, workers)
	if err != nil {
		return nil, NewMigrationSnapshotError(err)
	}

	l := &mapSnapshot{
		Payloads: pm,
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

func newIndexMapSnapshot(
	log zerolog.Logger,
	payloads []*ledger.Payload,
	workers int,
) (*indexMapSnapshot, error) {
	payloadsCopy := make([]*ledger.Payload, len(payloads))
	copy(payloadsCopy, payloads)

	im, err := createLargeIndexMap(log, payloadsCopy, workers)
	if err != nil {
		return nil, NewMigrationSnapshotError(err)
	}

	l := &indexMapSnapshot{
		reverseMap: im,
		payloads:   payloadsCopy,
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

func workersToUse(workers int, payloads []*ledger.Payload) int {
	// limit the number of workers to prevent too many small jobs
	const minPayloadsPerWorker = 10000
	if len(payloads)/minPayloadsPerWorker < workers {
		workers = len(payloads) / minPayloadsPerWorker
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}

func createLargeIndexMap(
	log zerolog.Logger,
	payloads []*ledger.Payload,
	workers int,
) (map[flow.RegisterID]int, error) {

	workers = workersToUse(workers, payloads)

	if workers == 1 {
		r := make(map[flow.RegisterID]int, len(payloads))
		for i, payload := range payloads {
			key, err := payload.Key()
			if err != nil {
				return nil, err
			}
			id, err := convert.LedgerKeyToRegisterID(key)
			if err != nil {
				return nil, err
			}
			r[id] = i
		}

		return r, nil
	}

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

	type result struct {
		minimap map[flow.RegisterID]int
		err     error
	}

	minimaps := make(chan result, workers)
	defer close(minimaps)

	// create the minimaps in parallel
	for i := 0; i < workers; i++ {
		start := i * len(payloads) / workers
		end := (i + 1) * len(payloads) / workers
		if end > len(payloads) {
			end = len(payloads)
		}

		go func(startIndex int, payloads []*ledger.Payload) {
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
				minimap[id] = startIndex + i
			}
			minimaps <- result{minimap, nil}
		}(start, payloads[start:end])

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

	return r.minimap, nil
}

// createLargeMap is the same as createLargeIndexMap, but it returns a map of RegisterID to index in the payloads array
// Generics are intentionally not used to avoid performance overhead
func createLargeMap(
	log zerolog.Logger,
	payloads []*ledger.Payload,
	workers int,
) (map[flow.RegisterID]*ledger.Payload, error) {

	workers = workersToUse(workers, payloads)

	if workers == 1 {
		r := make(map[flow.RegisterID]*ledger.Payload, len(payloads))
		for _, payload := range payloads {
			key, err := payload.Key()
			if err != nil {
				return nil, err
			}
			id, err := convert.LedgerKeyToRegisterID(key)
			if err != nil {
				return nil, err
			}
			r[id] = payload
		}

		return r, nil
	}

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

	type result struct {
		minimap map[flow.RegisterID]*ledger.Payload
		err     error
	}

	minimaps := make(chan result, workers)
	defer close(minimaps)

	// create the minimaps in parallel
	for i := 0; i < workers; i++ {
		start := i * len(payloads) / workers
		end := (i + 1) * len(payloads) / workers
		if end > len(payloads) {
			end = len(payloads)
		}

		go func(payloads []*ledger.Payload) {
			minimap := make(map[flow.RegisterID]*ledger.Payload, len(payloads))
			for _, payload := range payloads {
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
				minimap[id] = payload
			}
			minimaps <- result{minimap, nil}
		}(payloads[start:end])

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

	return r.minimap, nil
}
