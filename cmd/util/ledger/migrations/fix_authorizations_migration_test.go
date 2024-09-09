package migrations

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func newEntitlementSetAuthorizationFromTypeIDs(
	typeIDs []common.TypeID,
	setKind sema.EntitlementSetKind,
) interpreter.EntitlementSetAuthorization {
	return interpreter.NewEntitlementSetAuthorization(
		nil,
		func() []common.TypeID {
			return typeIDs
		},
		len(typeIDs),
		setKind,
	)
}

func TestFixAuthorizationsMigration(t *testing.T) {
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

	const contractCode = `
      access(all) contract Test {
          access(all) entitlement E1
          access(all) entitlement E2

          access(all) struct S {
              access(E1) fun f1() {}
              access(E2) fun f2() {}
              access(all) fun f3() {}
          }
      }
    `

	deployTX := flow.NewTransactionBody().
		SetScript([]byte(`
          transaction(code: String) {
              prepare(signer: auth(Contracts) &Account) {
                  signer.contracts.add(name: "Test", code: code.utf8)
              }
          }
        `)).
		AddAuthorizer(address).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractCode)))

	runDeployTx := NewTransactionBasedMigration(
		deployTX,
		chainID,
		log,
		expectedWriteAddresses,
	)
	err = runDeployTx(registersByAccount)
	require.NoError(t, err)

	setupTx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
              import Test from %s

              transaction {
                  prepare(signer: auth(Storage, Capabilities) &Account) {
                      signer.storage.save(Test.S(), to: /storage/s)

                      // Capability 1 was a public, unauthorized capability.
                      // It should lose entitlement E2
                      let cap1 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      assert(cap1.borrow() != nil)
                      signer.capabilities.publish(cap1, at: /public/s)

                      // Capability 2 was a public, unauthorized capability, stored nested in storage.
                      // It should lose entitlement E2
                      let cap2 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      assert(cap2.borrow() != nil)
                      signer.storage.save([cap2], to: /storage/caps2)

                      // Capability 3 was a private, authorized capability, stored nested in storage.
                      // It should keep entitlement E2
                      let cap3 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      assert(cap3.borrow() != nil)
                      signer.storage.save([cap3], to: /storage/caps3)

	                  // Capability 4 was a capability with unavailable accessible members, stored nested in storage.
	                  // It should keep entitlement E2
                      let cap4 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      assert(cap4.borrow() != nil)
                      signer.storage.save([cap4], to: /storage/caps4)
                  }
              }
            `,
			address.HexWithPrefix(),
		))).
		AddAuthorizer(address)

	runSetupTx := NewTransactionBasedMigration(
		setupTx,
		chainID,
		log,
		expectedWriteAddresses,
	)

	err = runSetupTx(registersByAccount)
	require.NoError(t, err)

	rwf := &testReportWriterFactory{}

	options := FixAuthorizationsMigrationOptions{
		ChainID: chainID,
		NWorker: nWorker,
	}

	testContractLocation := common.AddressLocation{
		Address: common.Address(address),
		Name:    "Test",
	}
	e1TypeID := testContractLocation.TypeID(nil, "Test.E1")

	fixedAuthorization := newEntitlementSetAuthorizationFromTypeIDs(
		[]common.TypeID{
			e1TypeID,
		},
		sema.Conjunction,
	)

	fixes := map[AccountCapabilityID]interpreter.Authorization{
		AccountCapabilityID{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: fixedAuthorization,
		AccountCapabilityID{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: fixedAuthorization,
	}

	migrations := NewFixAuthorizationsMigrations(
		log,
		rwf,
		fixes,
		options,
	)

	for _, namedMigration := range migrations {
		err = namedMigration.Migrate(registersByAccount)
		require.NoError(t, err)
	}

	reporter := rwf.reportWriters[fixAuthorizationsMigrationReporterName]
	require.NotNil(t, reporter)

	var entries []any

	for _, entry := range reporter.entries {
		switch entry := entry.(type) {
		case capabilityAuthorizationFixedEntry,
			capabilityControllerAuthorizationFixedEntry:

			entries = append(entries, entry)
		}
	}

	require.ElementsMatch(t,
		[]any{
			capabilityControllerAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "cap_con",
					Address: common.Address(address),
				},
				CapabilityID:     1,
				NewAuthorization: fixedAuthorization,
			},
			capabilityControllerAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "cap_con",
					Address: common.Address(address),
				},
				CapabilityID:     2,
				NewAuthorization: fixedAuthorization,
			},
			capabilityAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "public",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      1,
				NewAuthorization:  fixedAuthorization,
			},
			capabilityAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "storage",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      2,
				NewAuthorization:  fixedAuthorization,
			},
		},
		entries,
	)

	// Check account

	_, err = runScript(
		chainID,
		registersByAccount,
		fmt.Sprintf(
			//language=Cadence
			`
              import Test from %s

              access(all)
              fun main() {
                  let account = getAuthAccount<auth(Storage) &Account>(%[1]s)
                  // NOTE: capability can NOT be borrowed with E2 anymore
                  assert(account.capabilities.borrow<auth(Test.E1, Test.E2) &Test.S>(/public/s) == nil)
                  assert(account.capabilities.borrow<auth(Test.E1) &Test.S>(/public/s) != nil)

                  let caps2 = account.storage.copy<[Capability]>(from: /storage/caps2)!
                  // NOTE: capability can NOT be borrowed with E2 anymore
                  assert(caps2[0].borrow<auth(Test.E1, Test.E2) &Test.S>() == nil)
                  assert(caps2[0].borrow<auth(Test.E1) &Test.S>() != nil)

                  let caps3 = account.storage.copy<[Capability]>(from: /storage/caps3)!
                  // NOTE: capability can still be borrowed with E2
                  assert(caps3[0].borrow<auth(Test.E1, Test.E2) &Test.S>() != nil)
                  assert(caps3[0].borrow<auth(Test.E1) &Test.S>() != nil)

                  let caps4 = account.storage.copy<[Capability]>(from: /storage/caps4)!
                  // NOTE: capability can still be borrowed with E2
                  assert(caps4[0].borrow<auth(Test.E1, Test.E2) &Test.S>() != nil)
                  assert(caps4[0].borrow<auth(Test.E1) &Test.S>() != nil)
              }
            `,
			address.HexWithPrefix(),
		),
	)
	require.NoError(t, err)
}
