package migrations

import (
	"bytes"
	"context"
	"fmt"
	util2 "github.com/onflow/flow-go/module/util"
	"sort"
	"sync"
	"time"

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

		h, err := m.recursiveString(log, mr, domain, interpreter.StringStorageMapKey(key.(interpreter.StringAtreeValue)), value, hasher)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to string: %w", err)
		}

		hasher.Reset()

		hashes = append(hashes, h)
	}

	return hashes.SortAndHash(hasher)
}

func (m *CadenceDataValidationMigrations) recursiveString(
	log zerolog.Logger,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
	hasher hash.Hasher,
) ([]byte, error) {
	if mr.Address == mustHexToAddress("4eded0de73020ca5") {
		compositeValue, ok := value.(*interpreter.CompositeValue)
		if ok && string(compositeValue.TypeID()) == "A.4eded0de73020ca5.CricketMomentsShardedCollection.ShardedCollection" {
			return m.recursiveStringShardedCollection(log, mr, domain, key, value)
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

func getShardedCollectionMap(mr *migratorRuntime, value interpreter.Value) (*interpreter.DictionaryValue, error) {
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
	return shardedCollectionMap, nil
}

func getNftCollection(inter *interpreter.Interpreter, outerKey interpreter.Value, shardedCollectionMap *interpreter.DictionaryValue) (*interpreter.DictionaryValue, error) {
	value := shardedCollectionMap.GetKey(
		inter,
		interpreter.EmptyLocationRange,
		outerKey,
	)

	someCollection, ok := value.(*interpreter.SomeValue)
	if !ok {
		return nil, fmt.Errorf("expected collection to be *interpreter.SomeValue, got %T", value)
	}

	collection, ok := someCollection.InnerValue(
		inter,
		interpreter.EmptyLocationRange).(*interpreter.CompositeValue)
	if !ok {
		return nil, fmt.Errorf("expected inner collection to be *interpreter.CompositeValue, got %T", value)
	}

	ownedNFTsRaw := collection.GetField(
		inter,
		interpreter.EmptyLocationRange,
		"ownedNFTs",
	)
	if ownedNFTsRaw == nil {
		return nil, fmt.Errorf("expected ownedNFTs field")
	}
	ownedNFTs, ok := ownedNFTsRaw.(*interpreter.DictionaryValue)
	if !ok {
		return nil, fmt.Errorf("expected ownedNFTs to be *interpreter.DictionaryValue, got %T", ownedNFTsRaw)
	}
	return ownedNFTs, nil
}

type keyPair struct {
	shardedCollectionKey interpreter.Value
	nftCollectionKey     interpreter.Value
}

type hashWithKeys struct {
	key  uint64
	hash []byte
}

func (m *CadenceDataValidationMigrations) recursiveStringShardedCollection(
	log zerolog.Logger,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
) ([]byte, error) {

	sampler := log.Sample(util2.NewTimedSampler(30 * time.Second))
	log.Info().Msg("starting recursiveStringShardedCollection")
	defer log.Info().Msg("done with recursiveStringShardedCollection")

	// hash all values
	hashes, err := m.hashAllNftsInAllCollections(
		log,
		sampler,
		mr,
		domain,
		key,
		value,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to hash all values: %w", err)
	}

	return m.consolidateAllHashes(
		log,
		sampler,
		hashes,
	)

}

func (m *CadenceDataValidationMigrations) consolidateAllHashes(
	log zerolog.Logger,
	sampler zerolog.Logger,
	hashes map[uint64]sortableHashes,
) ([]byte, error) {

	log.Info().Msg("starting consolidateAllHashes")
	defer log.Info().Msg("done with consolidateAllHashes")

	ctx, c := context.WithCancelCause(context.Background())
	cancel := func(err error) {
		if err != nil {
			log.Info().Err(err).Msg("canceling context")
		}
		c(err)
	}
	defer cancel(nil)

	hashCollectionChan := make(chan uint64, m.nWorkers)
	hashedCollectionChan := make(chan []byte)
	finalHashes := make(sortableHashes, 0, 10_000_000)
	wg := sync.WaitGroup{}
	wg.Add(m.nWorkers)
	done := make(chan struct{})

	for i := 0; i < m.nWorkers; i++ {
		go func() {
			defer wg.Done()
			hasher := newHasher()

			for {
				select {
				case <-ctx.Done():
					return
				case i, ok := <-hashCollectionChan:
					if !ok {
						return
					}
					l := hashes[i]

					h, err := l.SortAndHash(hasher)
					if err != nil {
						cancel(fmt.Errorf("failed to write hash: %w", err))

						return
					}
					hashedCollectionChan <- h
				}
			}
		}()
	}

	go func() {
		defer close(done)
		defer log.Info().Msg("finished collecting final hashes")

		for {
			select {
			case <-ctx.Done():
				return
			case h, ok := <-hashedCollectionChan:
				if !ok {
					return
				}
				finalHashes = append(finalHashes, h)
			}
		}
	}()

	go func() {
		defer close(hashCollectionChan)
		defer log.Info().Msg("finished dispatching final hashes")

		for k := range hashes {
			select {
			case <-ctx.Done():
				return
			case hashCollectionChan <- k:
			}
		}
	}()

	wg.Wait()
	close(hashedCollectionChan)
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing values together")
		return nil, ctx.Err()
	}

	hasher := newHasher()
	h, err := finalHashes.SortAndHash(hasher)
	if err != nil {
		return nil, fmt.Errorf("failed to write hash: %w", err)
	}

	return h, nil
}

func (m *CadenceDataValidationMigrations) hashAllNftsInAllCollections(
	log zerolog.Logger,
	sampler zerolog.Logger,
	mr *migratorRuntime,
	domain string,
	key interpreter.StorageMapKey,
	value interpreter.Value,
) (map[uint64]sortableHashes, error) {

	log.Info().Msg("starting consolidateAllHashes")
	defer log.Info().Msg("done with consolidateAllHashes")

	ctx, c := context.WithCancelCause(context.Background())
	cancel := func(err error) {
		if err != nil {
			log.Info().Err(err).Msg("canceling context")
		}
		c(err)
	}
	defer cancel(nil)

	keyPairChan := make(chan keyPair, m.nWorkers)
	hashChan := make(chan hashWithKeys, m.nWorkers)
	hashes := make(map[uint64]sortableHashes)
	wg := sync.WaitGroup{}
	wg.Add(m.nWorkers)

	// workers for hashing
	for i := 0; i < m.nWorkers; i++ {
		go func() {
			defer wg.Done()

			storageMap := mr.GetReadOnlyStorage().GetStorageMap(mr.Address, domain, false)
			storageMapValue := storageMap.ReadValue(&util.NopMemoryGauge{}, key)

			hashNFTWorker(sampler, ctx, cancel, mr, storageMapValue, keyPairChan, hashChan)
		}()
	}

	// worker for collecting hashes
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer log.Info().Msg("finished collecting hashes")
		for {
			select {
			case <-ctx.Done():
				return
			case hashed, ok := <-hashChan:
				if !ok {
					return
				}
				_, ok = hashes[hashed.key]
				if !ok {
					hashes[hashed.key] = make(sortableHashes, 0, 1_000_000)
				}
				hashes[hashed.key] = append(hashes[hashed.key], hashed.hash)
			}
		}
	}()

	// worker for dispatching values to hash
	go func() {
		defer close(keyPairChan)
		defer log.Info().Msg("finished dispatching values to hash")

		shardedCollectionMap, err := getShardedCollectionMap(mr, value)
		if err != nil {
			cancel(err)
			return
		}

		log.Info().
			Int("shardedCollectionMapLen", shardedCollectionMap.Count()).
			Msg("shardedCollectionMapLen")

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

			ownedNFTs, err := getNftCollection(mr.Interpreter, outerKey, shardedCollectionMap)
			if err != nil {
				cancel(err)
				return
			}

			ownedNFTsIterator := ownedNFTs.Iterator()
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

				keyPairChan <- keyPair{
					nftCollectionKey:     innerKey,
					shardedCollectionKey: outerKey,
				}
			}
		}
	}()

	wg.Wait()
	log.Info().Msg("finished hashing values")
	close(hashChan)
	<-done

	if ctx.Err() != nil {
		log.Info().Err(ctx.Err()).Msg("context error when hashing individual values")
		return nil, ctx.Err()
	}
	return hashes, nil
}

func hashNFTWorker(
	sample zerolog.Logger,
	ctx context.Context,
	cancel context.CancelCauseFunc,
	mr *migratorRuntime,
	storageMapValue interpreter.Value,
	keyPairChan <-chan keyPair,
	hashChan chan<- hashWithKeys,
) {
	hasher := newHasher()

	shardedCollectionMap, err := getShardedCollectionMap(mr, storageMapValue)
	if err != nil {
		cancel(err)
		return
	}
	hashed := 0

	for {
		select {
		case <-ctx.Done():
			return
		case keyPair, ok := <-keyPairChan:
			if !ok {
				return
			}
			s := ""

			ownedNFTs, err := getNftCollection(
				mr.Interpreter,
				keyPair.shardedCollectionKey,
				shardedCollectionMap,
			)
			if err != nil {
				cancel(err)
				return
			}

			value := ownedNFTs.GetKey(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				keyPair.nftCollectionKey,
			)

			err = capturePanic(func() {
				s = value.RecursiveString(interpreter.SeenReferences{})
			})
			if err != nil {
				cancel(err)
				return
			}
			hash := hasher.ComputeHash([]byte(s))
			hasher.Reset()

			uintKey := uint64(keyPair.shardedCollectionKey.(interpreter.UInt64Value))

			hashChan <- hashWithKeys{
				key:  uintKey,
				hash: hash,
			}
			hashed++
			sample.Info().Int("hashed", hashed).Msg("Hashing worker running")
		}
	}
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

func (s sortableHashes) SortAndHash(hasher hash.Hasher) ([]byte, error) {
	defer hasher.Reset()
	sort.Sort(s)
	for _, h := range s {
		_, err := hasher.Write(h)
		if err != nil {
			return nil, fmt.Errorf("failed to write hash: %w", err)
		}

	}
	return hasher.SumHash(), nil
}

type cadenceDataValidationReportEntry struct {
	Address string `json:"address"`
	Problem string `json:"problem"`
}
