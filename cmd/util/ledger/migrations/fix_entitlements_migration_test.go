package migrations

import (
	"strings"
	"testing"

	"github.com/onflow/cadence/runtime/common"
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
                  signer.storage.save([cap2], to: /storage/caps1)

                  // Capability 3 was a private, authorized capability, stored nested in storage.
                  // It should keep its entitlement
                  let cap3 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap3], to: /storage/caps2)
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

	// Capability 1 was a public, unauthorized capability.
	// It should lose its entitlement
	//
	// Capability 2 was a public, unauthorized capability, stored nested in storage.
	// It should lose its entitlement
	//
	// Capability 3 was a private, authorized capability, stored nested in storage.
	// It should keep its entitlement

	publicLinkReport := PublicLinkReport{
		{
			Address:    common.Address(address),
			Identifier: "ints",
		}: {
			BorrowType:        "&[Int]",
			AccessibleMembers: []string{},
		},
		{
			Address:    common.Address(address),
			Identifier: "ints2",
		}: {
			BorrowType:        "&[Int]",
			AccessibleMembers: []string{},
		},
	}

	publicLinkMigrationReport := PublicLinkMigrationReport{
		{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: "ints",
		{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: "ints2",
	}

	migrations := NewFixEntitlementsMigrations(
		log,
		rwf,
		publicLinkReport,
		publicLinkMigrationReport,
		options,
	)

	for _, namedMigration := range migrations {
		err = namedMigration.Migrate(registersByAccount)
		require.NoError(t, err)
	}

	// TODO: validate
}

func TestReadPublicLinkMigrationReport(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(`
      [
        {"kind":"link-migration-success","account_address":"0x1","path":"/public/foo","capability_id":1},
        {"kind":"link-migration-success","account_address":"0x2","path":"/private/bar","capability_id":2}
      ]
    `)

	mapping, err := ReadPublicLinkMigrationReport(reader)
	require.NoError(t, err)

	require.Equal(t,
		PublicLinkMigrationReport{
			{
				Address:      common.MustBytesToAddress([]byte{0x1}),
				CapabilityID: 1,
			}: "foo",
		},
		mapping,
	)
}

func TestReadLinkReport(t *testing.T) {
	t.Parallel()

	reader := strings.NewReader(`
      [
        {"address":"0x1","identifier":"foo","linkType":"&Foo","accessibleMembers":["foo"]},
        {"address":"0x2","identifier":"bar","linkType":"&Bar","accessibleMembers":null}
      ]
    `)

	mapping, err := ReadPublicLinkReport(reader)
	require.NoError(t, err)

	require.Equal(t,
		PublicLinkReport{
			{
				Address:    common.MustBytesToAddress([]byte{0x1}),
				Identifier: "foo",
			}: {
				BorrowType:        "&Foo",
				AccessibleMembers: []string{"foo"},
			},
			{
				Address:    common.MustBytesToAddress([]byte{0x2}),
				Identifier: "bar",
			}: {
				BorrowType:        "&Bar",
				AccessibleMembers: nil,
			},
		},
		mapping,
	)
}
