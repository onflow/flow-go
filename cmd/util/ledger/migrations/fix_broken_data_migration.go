package migrations

import (
	"context"
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/onflow/cadence/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/atree"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// ShouldFixBrokenCompositeKeyedDictionary returns true if the given value is a dictionary with a composite key type.
//
// It is useful for use with atree's PersistentSlabStorage.FixLoadedBrokenReferences.
//
// NOTE: The intended use case is to enable migration programs in onflow/flow-go to fix broken references.
// As of April 2024, only 10 registers in testnet (not mainnet) were found to have broken references,
// and they seem to have resulted from a bug that was fixed 2 years ago by https://github.com/onflow/cadence/pull/1565.
func ShouldFixBrokenCompositeKeyedDictionary(atreeValue atree.Value) bool {
	orderedMap, ok := atreeValue.(*atree.OrderedMap)
	if !ok {
		return false
	}

	dictionaryStaticType, ok := orderedMap.Type().(*interpreter.DictionaryStaticType)
	if !ok {
		return false
	}

	_, ok = dictionaryStaticType.KeyType.(*interpreter.CompositeStaticType)
	return ok
}

type FixSlabsWithBrokenReferencesMigration struct {
	log            zerolog.Logger
	rw             reporters.ReportWriter
	outputDir      string
	accountsToFix  map[common.Address]struct{}
	nWorkers       int
	mutex          sync.Mutex
	brokenPayloads []*ledger.Payload
	payloadsFile   string
}

var _ AccountBasedMigration = &FixSlabsWithBrokenReferencesMigration{}

const fixSlabsWithBrokenReferencesName = "fix-slabs-with-broken-references"

func NewFixBrokenReferencesInSlabsMigration(
	outputDir string,
	rwf reporters.ReportWriterFactory,
	accountsToFix map[common.Address]struct{},
) *FixSlabsWithBrokenReferencesMigration {
	return &FixSlabsWithBrokenReferencesMigration{
		outputDir:      outputDir,
		rw:             rwf.ReportWriter(fixSlabsWithBrokenReferencesName),
		accountsToFix:  accountsToFix,
		brokenPayloads: make([]*ledger.Payload, 0, 10),
	}
}

func (m *FixSlabsWithBrokenReferencesMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	nWorkers int,
) error {
	m.log = log.
		With().
		Str("migration", fixSlabsWithBrokenReferencesName).
		Logger()
	m.nWorkers = nWorkers

	return nil
}

func (m *FixSlabsWithBrokenReferencesMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	if _, exist := m.accountsToFix[address]; !exist {
		return nil
	}

	migrationRuntime := NewBasicMigrationRuntime(accountRegisters)
	storage := migrationRuntime.Storage

	// Load all atree registers in storage
	err := util.LoadAtreeSlabsInStorage(storage, accountRegisters, m.nWorkers)
	if err != nil {
		return err
	}

	// Fix broken references
	fixedStorageIDs, skippedStorageIDs, err :=
		storage.FixLoadedBrokenReferences(ShouldFixBrokenCompositeKeyedDictionary)
	if err != nil {
		return err
	}

	if len(skippedStorageIDs) > 0 {
		m.log.Warn().
			Str("account", address.HexWithPrefix()).
			Msgf("skipped slabs with broken references: %v", skippedStorageIDs)
	}

	if len(fixedStorageIDs) == 0 {
		m.log.Warn().
			Str("account", address.HexWithPrefix()).
			Msgf("did not fix any slabs with broken references")

		return nil
	}

	m.log.Log().
		Str("account", address.HexWithPrefix()).
		Msgf("fixed %d slabs with broken references", len(fixedStorageIDs))

	// Save broken payloads to save to payload file later
	brokenPayloads, err := getAtreePayloadsByID(accountRegisters, fixedStorageIDs)
	if err != nil {
		return err
	}

	m.mergeBrokenPayloads(brokenPayloads)

	err = storage.NondeterministicFastCommit(m.nWorkers)
	if err != nil {
		return fmt.Errorf("failed to commit storage: %w", err)
	}

	// Commit/finalize the transaction

	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = migrationRuntime.Commit(expectedAddresses, m.log)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	// Log fixed payloads
	fixedPayloads, err := getAtreePayloadsByID(accountRegisters, fixedStorageIDs)
	if err != nil {
		return err
	}

	m.rw.Write(fixedSlabsWithBrokenReferences{
		Account:        address.Hex(),
		BrokenPayloads: brokenPayloads,
		FixedPayloads:  fixedPayloads,
	})

	return nil
}

func (m *FixSlabsWithBrokenReferencesMigration) mergeBrokenPayloads(payloads []*ledger.Payload) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.brokenPayloads = append(m.brokenPayloads, payloads...)
}

func (m *FixSlabsWithBrokenReferencesMigration) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	err := m.writeBrokenPayloads()
	if err != nil {
		return fmt.Errorf("failed to write broken payloads to file: %w", err)
	}

	return nil
}

func (m *FixSlabsWithBrokenReferencesMigration) writeBrokenPayloads() error {

	m.payloadsFile = path.Join(
		m.outputDir,
		fmt.Sprintf("broken_%d.payloads", int32(time.Now().Unix())),
	)

	writtenPayloadCount, err := util.CreatePayloadFile(
		m.log,
		m.payloadsFile,
		m.brokenPayloads,
		nil,
		true,
	)

	if err != nil {
		return fmt.Errorf("failed to write all broken payloads to file: %w", err)
	}

	if writtenPayloadCount != len(m.brokenPayloads) {
		return fmt.Errorf(
			"failed to write all broken payloads to file: expected %d, got %d",
			len(m.brokenPayloads),
			writtenPayloadCount,
		)
	}

	return nil
}

func getAtreePayloadsByID(
	registers *registers.AccountRegisters,
	ids map[atree.SlabID][]atree.SlabID,
) (
	[]*ledger.Payload,
	error,
) {
	outputPayloads := make([]*ledger.Payload, 0, len(ids))

	owner := registers.Owner()

	keys := make([]string, 0, len(ids))
	err := registers.ForEachKey(func(key string) error {

		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		slabID := atree.NewSlabID(
			atree.Address([]byte(owner)),
			atree.SlabIndex([]byte(key[1:])),
		)

		_, ok := ids[slabID]
		if !ok {
			return nil
		}

		keys = append(keys, key)

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)

	for _, key := range keys {
		value, err := registers.Get(owner, key)
		if err != nil {
			return nil, err
		}

		ledgerKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: owner,
			Key:   key,
		})
		payload := ledger.NewPayload(ledgerKey, value)
		outputPayloads = append(outputPayloads, payload)
	}

	return outputPayloads, nil
}

type fixedSlabsWithBrokenReferences struct {
	Account        string            `json:"account"`
	BrokenPayloads []*ledger.Payload `json:"broken_payloads"`
	FixedPayloads  []*ledger.Payload `json:"fixed_payloads"`
}
