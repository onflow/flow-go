package migrations

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/onflow/cadence/migrations"
	"github.com/rs/zerolog"

	"github.com/onflow/atree"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

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
	err := loadAtreeSlabsInStorage(storage, accountRegisters)
	if err != nil {
		return err
	}

	// Fix broken references
	fixedStorageIDs, skippedStorageIDs, err :=
		storage.FixLoadedBrokenReferences(migrations.ShouldFixBrokenCompositeKeyedDictionary)
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

	err = storage.FastCommit(m.nWorkers)
	if err != nil {
		return fmt.Errorf("failed to commit storage: %w", err)
	}

	// Finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes to the original payloads.
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply changes to account registers: %w", err)
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

	err := registers.ForEach(func(owner string, key string, value []byte) error {

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

		ledgerKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: owner,
			Key:   key,
		})
		payload := ledger.NewPayload(ledgerKey, value)
		outputPayloads = append(outputPayloads, payload)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return outputPayloads, nil
}

type fixedSlabsWithBrokenReferences struct {
	Account        string            `json:"account"`
	BrokenPayloads []*ledger.Payload `json:"broken_payloads"`
	FixedPayloads  []*ledger.Payload `json:"fixed_payloads"`
}
