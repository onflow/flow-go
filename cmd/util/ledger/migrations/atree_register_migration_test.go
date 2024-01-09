package migrations_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

func TestAtreeRegisterMigration(t *testing.T) {
	log := zerolog.New(zerolog.NewTestWriter(t))
	dir := t.TempDir()

	validation := migrations.NewCadenceDataValidationMigrations(
		reporters.NewReportFileWriterFactory(dir, log),
		2,
	)

	// Localnet v0.31 was used to produce an execution state that can be used for the tests.
	t.Run(
		"test v0.31 state",
		testWithExistingState(

			log,
			"test-data/bootstrapped_v0.31",
			migrations.CreateAccountBasedMigration(log, 2,
				[]migrations.AccountBasedMigration{
					validation.PreMigration(),
					migrations.NewAtreeRegisterMigrator(reporters.NewReportFileWriterFactory(dir, log), true, false),
					validation.PostMigration(),
				},
			),
			func(t *testing.T, oldPayloads []*ledger.Payload, newPayloads []*ledger.Payload) {

				oldPayloadsMap := make(map[flow.RegisterID]*ledger.Payload, len(oldPayloads))

				for _, payload := range oldPayloads {
					key, err := payload.Key()
					require.NoError(t, err)
					id, err := convert.LedgerKeyToRegisterID(key)
					require.NoError(t, err)
					oldPayloadsMap[id] = payload
				}

				newPayloadsMap := make(map[flow.RegisterID]*ledger.Payload, len(newPayloads))

				for _, payload := range newPayloads {
					key, err := payload.Key()
					require.NoError(t, err)
					id, err := convert.LedgerKeyToRegisterID(key)
					require.NoError(t, err)
					newPayloadsMap[id] = payload
				}

				for key, payload := range newPayloadsMap {
					value := newPayloadsMap[key].Value()

					// TODO: currently the migration does not change the payload values because
					// the atree version is not changed. This should be changed in the future.
					require.Equal(t, payload.Value(), value)
				}

				// commented out helper code to dump the payloads to csv files for manual inspection
				//dumpPayloads := func(n string, payloads []ledger.Payload) {
				//	f, err := os.Create(n)
				//	require.NoError(t, err)
				//
				//	defer f.Close()
				//
				//	for _, payload := range payloads {
				//		key, err := payload.Key()
				//		require.NoError(t, err)
				//		_, err = f.WriteString(fmt.Sprintf("%x,%s\n", key.String(), payload.Value()))
				//		require.NoError(t, err)
				//	}
				//}
				//dumpPayloads("old.csv", oldPayloads)
				//dumpPayloads("new.csv", newPayloads)

			},
		),
	)

}

func testWithExistingState(
	log zerolog.Logger,
	inputDir string,
	migration ledger.Migration,
	f func(
		t *testing.T,
		oldPayloads []*ledger.Payload,
		newPayloads []*ledger.Payload,
	),
) func(t *testing.T) {
	return func(t *testing.T) {
		diskWal, err := wal.NewDiskWAL(
			log,
			nil,
			metrics.NewNoopCollector(),
			inputDir,
			complete.DefaultCacheSize,
			pathfinder.PathByteSize,
			wal.SegmentSize,
		)
		require.NoError(t, err)

		led, err := complete.NewLedger(
			diskWal,
			complete.DefaultCacheSize,
			&metrics.NoopCollector{},
			log,
			complete.DefaultPathFinderVersion)
		require.NoError(t, err)

		var oldPayloads []*ledger.Payload

		// we sandwitch the migration between two identity migrations
		// so we can capture the Payloads before and after the migration
		var mig = []ledger.Migration{
			func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
				oldPayloads = make([]*ledger.Payload, len(payloads))
				copy(oldPayloads, payloads)
				return payloads, nil
			},

			migration,

			func(newPayloads []*ledger.Payload) ([]*ledger.Payload, error) {

				f(t, oldPayloads, newPayloads)

				return newPayloads, nil
			},
		}

		newState, err := led.MostRecentTouchedState()
		require.NoError(t, err)

		_, err = led.MigrateAt(
			newState,
			mig,
			complete.DefaultPathFinderVersion,
		)
		require.NoError(t, err)
	}
}
