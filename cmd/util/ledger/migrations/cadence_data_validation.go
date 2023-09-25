package migrations

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger"
)

type CadenceDataValidationMigrations struct {
	rwf reporters.ReportWriterFactory

	mu     sync.RWMutex
	hashes map[common.Address][]byte

	nWorkers int
}

func NewCadenceDataValidationMigrations(
	rwf reporters.ReportWriterFactory,
	nWorkers int,
) *CadenceDataValidationMigrations {
	return &CadenceDataValidationMigrations{
		rwf:      rwf,
		hashes:   make(map[common.Address][]byte, 40_000_000),
		nWorkers: nWorkers,
	}
}

func (m *CadenceDataValidationMigrations) PreMigration() AccountBasedMigration {
	return &preMigration{
		v: m,
	}
}

func (m *CadenceDataValidationMigrations) PostMigration() AccountBasedMigration {
	return &postMigration{
		rwf: m.rwf,
		v:   m,
	}
}

func (m *CadenceDataValidationMigrations) set(key common.Address, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hashes[key] = value
}

func (m *CadenceDataValidationMigrations) get(key common.Address) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.hashes[key]
	return value, ok
}

func (m *CadenceDataValidationMigrations) delete(address common.Address) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.hashes, address)
}

type preMigration struct {
	log zerolog.Logger

	v *CadenceDataValidationMigrations
}

var _ AccountBasedMigration = (*preMigration)(nil)

func (m *preMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.With().Str("component", "CadenceDataValidationPreMigration").Logger()

	return nil
}

func (m *preMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
	if address == common.ZeroAddress {
		return payloads, nil
	}
	if address != mustHexToAddress("4eded0de73020ca5") {
		return payloads, nil
	}
	if _, ok := knownProblematicAccounts[address]; ok {
		m.log.Error().
			Hex("address", address[:]).
			Msg("skipping problematic account")
		return payloads, nil
	}

	hash, err := m.v.hashAccountCadenceValues(m.log, address, payloads)
	if err != nil {
		m.log.Info().
			Err(err).
			Hex("address", address[:]).
			Msg("failed to hash cadence values")
		return payloads, nil
	}

	m.v.set(address, hash)

	return payloads, nil
}

type postMigration struct {
	log zerolog.Logger

	rwf reporters.ReportWriterFactory
	rw  reporters.ReportWriter

	v *CadenceDataValidationMigrations
}

var _ AccountBasedMigration = &postMigration{}

func (m *postMigration) Close() error {
	for address := range m.v.hashes {
		m.log.Error().
			Hex("address", address[:]).
			Msg("cadence values missing after migration")

		m.rw.Write(
			cadenceDataValidationReportEntry{

				Address: address.Hex(),
				Problem: "cadence values missing after migration",
			},
		)
	}
	return nil
}

func (m *postMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.With().Str("component", "CadenceDataValidationPostMigration").Logger()
	m.rw = m.rwf.ReportWriter("cadence_data_validation")

	return nil
}

func (m *postMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
	if address == common.ZeroAddress {
		return payloads, nil
	}
	if address != mustHexToAddress("4eded0de73020ca5") {
		return payloads, nil
	}

	if _, ok := knownProblematicAccounts[address]; ok {
		m.log.Error().
			Hex("address", address[:]).
			Msg("skipping problematic account")
		return payloads, nil
	}

	newHash, err := m.v.hashAccountCadenceValues(m.log, address, payloads)
	if err != nil {
		m.log.Info().
			Err(err).
			Hex("address", address[:]).
			Msg("failed to hash cadence values")
		return payloads, nil
	}

	hash, ok := m.v.get(address)

	if !ok {
		m.log.Error().
			Hex("address", address[:]).
			Msg("cadence values missing before migration")

		m.rw.Write(
			cadenceDataValidationReportEntry{

				Address: address.Hex(),
				Problem: "cadence values missing before migration",
			},
		)
	}
	if !bytes.Equal(hash, newHash) {
		m.log.Error().
			Hex("address", address[:]).
			Msg("cadence values mismatch")

		m.rw.Write(
			cadenceDataValidationReportEntry{

				Address: address.Hex(),
				Problem: "cadence values mismatch",
			},
		)
	}

	// remove the address from the map so we can check if there are any
	// missing addresses
	m.v.delete(address)

	return payloads, nil
}

func (m *CadenceDataValidationMigrations) hashAccountCadenceValues(
	log zerolog.Logger,
	address common.Address,
	payloads []*ledger.Payload,
) ([]byte, error) {
	hasher := newHasher()
	mr, err := newMigratorRuntime(address, payloads)
	if err != nil {
		return nil, err
	}

	// iterate through all domains and migrate them
	for _, domain := range domains {
		domainHash, err := m.hashDomainCadenceValues(log, mr, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to hash storage domain %s : %w", domain, err)
		}
		_, err = hasher.Write(domainHash)
		if err != nil {
			return nil, fmt.Errorf("failed to write hash: %w", err)
		}
	}

	return hasher.SumHash(), nil
}

func (m *CadenceDataValidationMigrations) hashDomainCadenceValues(
	log zerolog.Logger,
	mr *migratorRuntime,
	domain string,
) ([]byte, error) {
	hasher := newHasher()
	var storageMap *interpreter.StorageMap
	err := capturePanic(func() {
		storageMap = mr.Storage.GetStorageMap(mr.Address, domain, false)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get storage map: %w", err)
	}
	if storageMap == nil {
		// no storage for this domain
		return nil, nil
	}

	hashes := make(sortableHashes, 0, storageMap.Count())

	iterator := storageMap.Iterator(util.NopMemoryGauge{})
	for {
		key, value := iterator.Next()
		if key == nil {
			break
		}

		h, err := m.recursiveString(log, mr, value, hasher)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string: %w", err)
		}

		//cadenceValue, err := runtime.ExportValue(value, mr.Interpreter, interpreter.EmptyLocationRange)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to export value: %w", err)
		//}
		//
		//encoded, err := jsoncdc.Encode(cadenceValue)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to encode value: %w", err)
		//}

		hasher.Reset()

		hashes = append(hashes, h)
	}

	// order the hashes since iteration order is not deterministic
	sort.Sort(hashes)

	for _, h := range hashes {
		_, err := hasher.Write(h)
		if err != nil {
			return nil, fmt.Errorf("failed to write hash: %w", err)
		}
	}

	return hasher.SumHash(), nil

}

func (m *CadenceDataValidationMigrations) recursiveString(
	log zerolog.Logger,
	mr *migratorRuntime,
	value interpreter.Value,
	hasher hash.Hasher,
) ([]byte, error) {
	if mr.Address == mustHexToAddress("4eded0de73020ca5") {
		compositeValue, ok := value.(*interpreter.CompositeValue)
		if ok && string(compositeValue.TypeID()) == "A.4eded0de73020ca5.CricketMomentsShardedCollection.ShardedCollection" {
			return m.recursiveStringShardedCollection(log, mr, value)
		}
	}

	var s string
	err := capturePanic(
		func() {
			s = value.RecursiveString(interpreter.SeenReferences{})
		})
	if err != nil {
		return nil, fmt.Errorf("failed to convert value to string: %w", err)
	}

	h := hasher.ComputeHash([]byte(s))
	return h, nil
}

func (m *CadenceDataValidationMigrations) recursiveStringShardedCollection(
	log zerolog.Logger,
	mr *migratorRuntime,
	value interpreter.Value,
) ([]byte, error) {

	shardedCollectionResource, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("expected *interpreter.CompositeValue, got %T", value)
	}
	shardedCollectionMapField := shardedCollectionResource.GetField(
		mr.Interpreter,
		interpreter.EmptyLocationRange,
		"collections",
	)
	if shardedCollectionMapField == nil {
		return nil, fmt.Errorf("expected collections field")
	}
	shardedCollectionMap, ok := shardedCollectionMapField.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected collections to be *interpreter.DictionaryValue, got %T", shardedCollectionMapField)
	}

	type valueWithKeys struct {
		outerKey interpreter.Value
		innerKey interpreter.Value
		value    interpreter.Value
	}

	type hashWithKeys struct {
		outerKey interpreter.Value
		innerKey interpreter.Value
		hash     []byte
	}

	ctx, c := context.WithCancelCause(context.Background())

	cancel := func(err error) {
		log.Info().Err(err).Msg("canceling context")
		c(err)
	}
	defer cancel(nil)

	hashifyChan := make(chan valueWithKeys, m.nWorkers)
	hashChan := make(chan hashWithKeys, m.nWorkers)
	wg := sync.WaitGroup{}
	wg.Add(m.nWorkers)

	for i := 0; i < m.nWorkers; i++ {
		hasher := newHasher()
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case hashify, ok := <-hashifyChan:
					if !ok {
						return
					}
					s := ""
					err := capturePanic(func() {
						s = hashify.value.RecursiveString(interpreter.SeenReferences{})
					})
					if err != nil {
						cancel(err)
						return
					}
					hash := hasher.ComputeHash([]byte(s))
					hasher.Reset()

					hashChan <- hashWithKeys{
						outerKey: hashify.outerKey,
						innerKey: hashify.innerKey,
						hash:     hash,
					}
				}
			}
		}()
	}

	go func() {
		shardedCollectionMapIterator := shardedCollectionMap.Iterator()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			outerKey := shardedCollectionMapIterator.NextKey(nil)
			if outerKey == nil {
				break
			}
			value := shardedCollectionMap.GetKey(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				outerKey,
			)

			collection, ok := value.(*interpreter.CompositeValue)
			if !ok {
				cancel(fmt.Errorf("expected collection to be *interpreter.CompositeValue, got %T", value))
				return
			}

			ownedNFTsRaw := collection.GetField(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				"ownedNFTs",
			)
			if ownedNFTsRaw == nil {
				cancel(fmt.Errorf("expected ownedNFTs field"))
				return
			}
			ownedNFTs, ok := ownedNFTsRaw.(*interpreter.DictionaryValue)
			if !ok {
				cancel(fmt.Errorf("expected ownedNFTs to be *interpreter.DictionaryValue, got %T", ownedNFTsRaw))
				return
			}

			ownedNFTsIterator := ownedNFTs.Iterator()
			keys := make([]interpreter.Value, 0, ownedNFTs.Count())
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				innerKey := ownedNFTsIterator.NextKey(nil)
				if innerKey == nil {
					break
				}
				value := ownedNFTs.GetKey(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					innerKey,
				)

				hashifyChan <- valueWithKeys{
					innerKey: innerKey,
					outerKey: outerKey,
					value:    value,
				}

				keys = append(keys, innerKey)
			}

			for _, key := range keys {
				ownedNFTs.Remove(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					key,
				)
			}
		}
		close(hashifyChan)
	}()

	done := make(chan struct{})
	hashes := make(map[interpreter.Value]sortableHashes)

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case clone, ok := <-hashChan:
				if !ok {
					return
				}
				l, ok := hashes[clone.outerKey]
				if !ok {
					l = make(sortableHashes, 0, 1_0000_000)
				}
				l = append(l, clone.hash)
				hashes[clone.outerKey] = l
			}
		}
	}()

	wg.Wait()
	close(hashChan)
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing individual values")
		return nil, ctx.Err()
	}

	inputChan := make(chan interpreter.Value, m.nWorkers)
	outputChan := make(chan []byte)
	wg.Add(m.nWorkers)
	done = make(chan struct{})

	for i := 0; i < m.nWorkers; i++ {
		go func() {
			defer wg.Done()
			hasher := newHasher()

			for {
				select {
				case <-ctx.Done():
					return
				case i, ok := <-inputChan:
					if !ok {
						return
					}
					l := hashes[i]
					sort.Sort(l)
					for _, h := range l {
						_, err := hasher.Write(h)
						if err != nil {
							cancel(fmt.Errorf("failed to write hash: %w", err))

							return
						}
					}
					outputChan <- hasher.SumHash()
					hasher.Reset()
				}
			}
		}()
	}

	outHashes := make(sortableHashes, 0, 10_000_000)

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-outputChan:
				if !ok {
					return
				}
				outHashes = append(outHashes, h)
			}
		}
	}()

	for k := range hashes {
		inputChan <- k
	}
	close(inputChan)
	wg.Wait()
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing values together")
		return nil, ctx.Err()
	}

	sort.Sort(outHashes)
	hasher := newHasher()

	for _, h := range outHashes {
		_, err := hasher.Write(h)
		if err != nil {
			return nil, fmt.Errorf("failed to write hash: %w", err)
		}

	}
	return hasher.SumHash(), nil
}

func newHasher() hash.Hasher {
	return hash.NewSHA3_256()
}

type sortableHashes [][]byte

func (s sortableHashes) Len() int {
	return len(s)
}

func (s sortableHashes) Less(i, j int) bool {
	return bytes.Compare(s[i], s[j]) < 0
}

func (s sortableHashes) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type cadenceDataValidationReportEntry struct {
	Address string `json:"address"`
	Problem string `json:"problem"`
}
