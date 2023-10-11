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

	if address != cricketMomentsAddress {
		// skip non-cricket-moments accounts for quicker testing
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

	if address != cricketMomentsAddress {
		// skip non-cricket-moments accounts for quicker testing
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
	if isCricketMomentsShardedCollection(mr, value) {
		log.Info().Msg("recursive string hash for cricket moments sharded collection")
		return recursiveStringShardedCollection(log, m.nWorkers, mr, domain, key, value)
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
