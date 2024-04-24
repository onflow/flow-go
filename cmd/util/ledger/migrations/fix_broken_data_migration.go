package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/atree"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type FixSlabsWithBrokenReferencesMigration struct {
	log           zerolog.Logger
	rw            reporters.ReportWriter
	accountsToFix map[common.Address]struct{}
	nWorkers      int
}

var _ AccountBasedMigration = &FixSlabsWithBrokenReferencesMigration{}

const fixSlabsWithBrokenReferencesName = "fix-slabs-with-broken-references"

func NewFixBrokenReferencesInSlabsMigration(
	rwf reporters.ReportWriterFactory,
	accountsToFix map[common.Address]struct{},
) *FixSlabsWithBrokenReferencesMigration {
	return &FixSlabsWithBrokenReferencesMigration{
		rw:            rwf.ReportWriter(fixSlabsWithBrokenReferencesName),
		accountsToFix: accountsToFix,
	}
}

func (m *FixSlabsWithBrokenReferencesMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
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
	oldPayloads []*ledger.Payload,
) (
	newPayloads []*ledger.Payload,
	err error,
) {

	if _, exist := m.accountsToFix[address]; !exist {
		return oldPayloads, nil
	}

	migrationRuntime, err := NewAtreeRegisterMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create cadence runtime: %w", err)
	}

	storage := migrationRuntime.Storage

	// Load all atree registers in storage
	err = loadAtreeSlabsInStorge(storage, oldPayloads)
	if err != nil {
		return nil, err
	}

	// Fix broken references
	fixedStorageIDs, skippedStorageIDs, err := storage.FixLoadedBrokenReferences(func(old atree.Value) bool {
		// TODO: Cadence may need to export functions to check type info, etc.
		return true
	})
	if err != nil {
		return nil, err
	}

	if len(skippedStorageIDs) > 0 {
		m.log.Warn().
			Str("account", address.Hex()).
			Msgf("skipped slabs with broken references: %v", skippedStorageIDs)
	}

	if len(fixedStorageIDs) == 0 {
		m.log.Warn().
			Str("account", address.Hex()).
			Msgf("did not fix any slabs with broken references")

		return oldPayloads, nil
	}

	m.log.Log().
		Str("account", address.Hex()).
		Msgf("fixed slabs with broken references: %v", fixedStorageIDs)

	err = storage.FastCommit(m.nWorkers)
	if err != nil {
		return nil, err
	}

	// Finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return nil, fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes to the original payloads.
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	newPayloads, err = migrationRuntime.Snapshot.ApplyChangesAndGetNewPayloads(
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return nil, err
	}

	// Log fixed payloads
	fixedPayloads := make([]*ledger.Payload, 0, len(fixedStorageIDs))
	for _, payload := range newPayloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if !registerID.IsSlabIndex() {
			continue
		}

		storageID := atree.NewStorageID(
			atree.Address([]byte(registerID.Owner)),
			atree.StorageIndex([]byte(registerID.Key[1:])),
		)

		if _, ok := fixedStorageIDs[storageID]; ok {
			fixedPayloads = append(fixedPayloads, payload)
		}
	}

	m.rw.Write(fixedSlabsWithBrokenReferences{
		Account:  address,
		Payloads: fixedPayloads,
	})

	return newPayloads, nil
}

func (m *FixSlabsWithBrokenReferencesMigration) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	return nil
}

type fixedSlabsWithBrokenReferences struct {
	Account  common.Address    `json:"account"`
	Payloads []*ledger.Payload `json:"payloads"`
}
