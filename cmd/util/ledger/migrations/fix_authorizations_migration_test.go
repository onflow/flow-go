package migrations

import (
	"fmt"
	"strings"
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

          access(all) entitlement E

          access(all) struct S {}
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

                      // Capability 1 was a public, unauthorized capability, which is now authorized.
                      // It should lose its entitlement
                      let cap1 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      assert(cap1.borrow() != nil)
                      signer.capabilities.publish(cap1, at: /public/s1)

                      // Capability 2 was a public, unauthorized capability, which is now authorized.
                      // It is currently only stored, nested, in storage, and is not published.
                      // It should lose its entitlement
                      let cap2 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      assert(cap2.borrow() != nil)
                      signer.storage.save([cap2], to: /storage/caps2)

                      // Capability 3 was a private, authorized capability.
                      // It is currently only stored, nested, in storage, and is not published.
                      // It should keep its entitlement
                      let cap3 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      assert(cap3.borrow() != nil)
                      signer.storage.save([cap3], to: /storage/caps3)

                      // Capability 4 was a private, authorized capability.
                      // It is currently both stored, nested, in storage, and is published.
                      // It should keep its entitlement
                      let cap4 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      assert(cap4.borrow() != nil)
                      signer.storage.save([cap4], to: /storage/caps4)
                      signer.capabilities.publish(cap4, at: /public/s4)

                      // Capability 5 was a public, unauthorized capability, which is still unauthorized.
                      // It is currently both stored, nested, in storage, and is published.
                      // There is no need to fix it.
                      let cap5 = signer.capabilities.storage.issue<&Test.S>(/storage/s)
                      assert(cap5.borrow() != nil)
                      signer.storage.save([cap5], to: /storage/caps5)
                      signer.capabilities.publish(cap5, at: /public/s5)
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

	options := Options{
		ChainID: chainID,
		NWorker: nWorker,
	}

	fixes := AuthorizationFixes{
		AccountCapabilityID{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: {},
		AccountCapabilityID{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: {},
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
				CapabilityID: 1,
			},
			capabilityControllerAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "cap_con",
					Address: common.Address(address),
				},
				CapabilityID: 2,
			},
			capabilityAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "public",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      1,
			},
			capabilityAuthorizationFixedEntry{
				StorageKey: interpreter.StorageKey{
					Key:     "storage",
					Address: common.Address(address),
				},
				CapabilityAddress: common.Address(address),
				CapabilityID:      2,
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
	             // NOTE: capability can NOT be borrowed with E anymore
	             assert(account.capabilities.borrow<auth(Test.E) &Test.S>(/public/s1) == nil)
	             assert(account.capabilities.borrow<&Test.S>(/public/s1) != nil)

	             let caps2 = account.storage.copy<[Capability]>(from: /storage/caps2)!
	             // NOTE: capability can NOT be borrowed with E anymore
	             assert(caps2[0].borrow<auth(Test.E) &Test.S>() == nil)
	             assert(caps2[0].borrow<&Test.S>() != nil)

	             let caps3 = account.storage.copy<[Capability]>(from: /storage/caps3)!
	             // NOTE: capability can still be borrowed with E
	             assert(caps3[0].borrow<auth(Test.E) &Test.S>() != nil)
	             assert(caps3[0].borrow<&Test.S>() != nil)

	             let caps4 = account.storage.copy<[Capability]>(from: /storage/caps4)!
	             // NOTE: capability can still be borrowed with E
	             assert(account.capabilities.borrow<auth(Test.E) &Test.S>(/public/s4) != nil)
	             assert(account.capabilities.borrow<&Test.S>(/public/s4) != nil)
	             assert(caps4[0].borrow<auth(Test.E) &Test.S>() != nil)
	             assert(caps4[0].borrow<&Test.S>() != nil)
	         }
	       `,
			address.HexWithPrefix(),
		),
	)
	require.NoError(t, err)
}

func TestReadAuthorizationFixes(t *testing.T) {
	t.Parallel()

	validContents := `
      [
        {"capability_address":"01","capability_id":4},
        {"capability_address":"02","capability_id":5},
        {"capability_address":"03","capability_id":6}
      ]
    `

	t.Run("unfiltered", func(t *testing.T) {

		t.Parallel()

		reader := strings.NewReader(validContents)

		mapping, err := ReadAuthorizationFixes(reader, nil)
		require.NoError(t, err)

		require.Equal(t,
			AuthorizationFixes{
				{
					Address:      common.MustBytesToAddress([]byte{0x1}),
					CapabilityID: 4,
				}: {},
				{
					Address:      common.MustBytesToAddress([]byte{0x2}),
					CapabilityID: 5,
				}: {},
				{
					Address:      common.MustBytesToAddress([]byte{0x3}),
					CapabilityID: 6,
				}: {},
			},
			mapping,
		)
	})

	t.Run("filtered", func(t *testing.T) {

		t.Parallel()

		address1 := common.MustBytesToAddress([]byte{0x1})
		address3 := common.MustBytesToAddress([]byte{0x3})

		addressFilter := map[common.Address]struct{}{
			address1: {},
			address3: {},
		}

		reader := strings.NewReader(validContents)

		mapping, err := ReadAuthorizationFixes(reader, addressFilter)
		require.NoError(t, err)

		require.Equal(t,
			AuthorizationFixes{
				{
					Address:      common.MustBytesToAddress([]byte{0x1}),
					CapabilityID: 4,
				}: {},
				{
					Address:      common.MustBytesToAddress([]byte{0x3}),
					CapabilityID: 6,
				}: {},
			},
			mapping,
		)
	})
}
