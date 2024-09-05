package migrations

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func TestPublicEntitlementMigration(t *testing.T) {
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
