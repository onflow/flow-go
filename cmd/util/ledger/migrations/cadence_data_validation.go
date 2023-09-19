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
}

func NewCadenceDataValidationMigrations(
	rwf reporters.ReportWriterFactory,
	nWorkers int,
) *CadenceDataValidationMigrations {
	return &CadenceDataValidationMigrations{
		rwf:    rwf,
		hashes: make(map[common.Address][]byte, 40_000_000),
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

	hash, err := hashAccountCadenceValues(address, payloads)
	if err != nil {
		return nil, err
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
			Msg("cadence values missing")

		m.rw.Write(
			cadenceDataValidationReportEntry{

				Address: address.Hex(),
				Problem: "cadence values missing",
			},
		)
	}
	return nil
}

func (m *postMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	nWorker int,
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

	newHash, err := hashAccountCadenceValues(address, payloads)
	if err != nil {
		return nil, err
	}

	hash, ok := m.v.get(address)

	if !ok {
		m.log.Error().
			Hex("address", address[:]).
			Msg("cadence values missing")

		m.rw.Write(
			cadenceDataValidationReportEntry{

				Address: address.Hex(),
				Problem: "cadence values missing",
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

func hashAccountCadenceValues(
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
		domainHash, err := hashDomainCadenceValues(mr, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to convert storage domain %s : %w", domain, err)
		}
		_, err = hasher.Write(domainHash)
		if err != nil {
			return nil, fmt.Errorf("failed to write hash: %w", err)
		}
	}

	return hasher.SumHash(), nil
}

func hashDomainCadenceValues(
	mr *migratorRuntime,
	domain string,
) ([]byte, error) {
	hasher := newHasher()

	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, false)
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

		// TODO: check if this is enough. We might need to switch to jsoncdc
		s := value.RecursiveString(interpreter.SeenReferences{})

		//cadenceValue, err := runtime.ExportValue(value, mr.Interpreter, interpreter.EmptyLocationRange)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to export value: %w", err)
		//}
		//
		//encoded, err := jsoncdc.Encode(cadenceValue)
		//if err != nil {
		//	return nil, fmt.Errorf("failed to encode value: %w", err)
		//}

		h := hasher.ComputeHash([]byte(s))
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
