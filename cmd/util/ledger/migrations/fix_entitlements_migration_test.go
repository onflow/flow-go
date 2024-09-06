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

	log := zerolog.New(zerolog.NewTestWriter(t))

	bootstrapPayloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	registersByAccount, err := registers.NewByAccountFromPayloads(bootstrapPayloads)
	require.NoError(t, err)

	tx := flow.NewTransactionBody().
		SetScript([]byte(`
          transaction {
              prepare(signer: auth(Storage, Capabilities) &Account) {
                  let cap = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap], to: /storage/caps)
              }
          }
        `)).
		AddAuthorizer(chain.ServiceAddress())

	setupTx := NewTransactionBasedMigration(
		tx,
		chainID,
		log,
		map[flow.Address]struct{}{
			chain.ServiceAddress(): {},
		},
	)

	err = setupTx(registersByAccount)
	require.NoError(t, err)

	rwf := &testReportWriterFactory{}

	options := FixEntitlementsMigrationOptions{
		ChainID: chainID,
		NWorker: nWorker,
	}

	migrations := NewFixEntitlementsMigrations(log, rwf, options)

	for _, namedMigration := range migrations {
		err = namedMigration.Migrate(registersByAccount)
		require.NoError(t, err)
	}

	// TODO: validate
}

func TestReadLinkMigrationReport(t *testing.T) {
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
		LinkMigrationReport{
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
