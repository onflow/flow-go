package migrations

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func TestFixEntitlementMigrations(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	chain := chainID.Chain()

	const nWorker = 2

	address, err := chain.AddressAtIndex(1000)
	require.NoError(t, err)

	require.Equal(t, "bf519681cdb888b1", address.Hex())

	log := zerolog.New(zerolog.NewTestWriter(t))

	bootstrapPayloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	registersByAccount, err := registers.NewByAccountFromPayloads(bootstrapPayloads)
	require.NoError(t, err)

	mr := NewBasicMigrationRuntime(registersByAccount)
	err = mr.Accounts.Create(nil, address)
	require.NoError(t, err)

	expectedWriteAddresses := map[flow.Address]struct{}{
		address: {},
	}

	err = mr.Commit(expectedWriteAddresses, log)
	require.NoError(t, err)

	tx := flow.NewTransactionBody().
		SetScript([]byte(`
          transaction {
              prepare(signer: auth(Storage, Capabilities) &Account) {
                  // Capability 1 was a public, unauthorized capability.
                  // It should lose its entitlement
                  let cap1 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.capabilities.publish(cap1, at: /public/ints)

                  // Capability 2 was a public, unauthorized capability, stored nested in storage.
                  // It should lose its entitlement
                  let cap2 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap2], to: /storage/caps2)

                  // Capability 3 was a private, authorized capability, stored nested in storage.
                  // It should keep its entitlement
                  let cap3 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap3], to: /storage/caps3)

	               // Capability 4 was a capability with unavailable accessible members, stored nested in storage.
	               // It should keep its entitlement
                  let cap4 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap4], to: /storage/caps4)
              }
          }
        `)).
		AddAuthorizer(address)

	setupTx := NewTransactionBasedMigration(
		tx,
		chainID,
		log,
		expectedWriteAddresses,
	)

	err = setupTx(registersByAccount)
	require.NoError(t, err)

	rwf := &testReportWriterFactory{}

	options := FixEntitlementsMigrationOptions{
		ChainID: chainID,
		NWorker: nWorker,
	}

	fixes := map[AccountCapabilityControllerID]interpreter.Authorization{
		{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: interpreter.UnauthorizedAccess,
		{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: interpreter.UnauthorizedAccess,
	}

	migrations := NewFixEntitlementsMigrations(
		log,
		rwf,
		fixes,
		options,
	)

	for _, namedMigration := range migrations {
		err = namedMigration.Migrate(registersByAccount)
		require.NoError(t, err)
	}

	reporter := rwf.reportWriters[fixEntitlementsMigrationReporterName]
	require.NotNil(t, reporter)

	var entries []any

	for _, entry := range reporter.entries {
		switch entry := entry.(type) {
		case capabilityEntitlementsFixedEntry,
			capabilityControllerEntitlementsFixedEntry:

			entries = append(entries, entry)
		}
	}

	require.ElementsMatch(t,
		[]any{
			capabilityControllerEntitlementsFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "cap_con",
					Address: common.Address(address),
				},
				CapabilityID:     1,
				NewAuthorization: interpreter.UnauthorizedAccess,
			},
			capabilityControllerEntitlementsFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "cap_con",
					Address: common.Address(address),
				},
				CapabilityID:     2,
				NewAuthorization: interpreter.UnauthorizedAccess,
			},
			capabilityEntitlementsFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "public",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      1,
				NewAuthorization:  interpreter.UnauthorizedAccess,
			},
			capabilityEntitlementsFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "storage",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      2,
				NewAuthorization:  interpreter.UnauthorizedAccess,
			},
		},
		entries,
	)
}
