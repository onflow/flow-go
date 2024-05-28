package migrations

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func TestMigrationMetricsCollection(t *testing.T) {

	t.Parallel()

	t.Run("contract not staged", func(t *testing.T) {

		t.Parallel()

		// Get the old payloads
		payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
		require.NoError(t, err)

		registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
		require.NoError(t, err)

		rwf := &testReportWriterFactory{}

		logWriter := &writer{}
		logger := zerolog.New(logWriter).Level(zerolog.InfoLevel)

		const nWorker = 2

		const chainID = flow.Emulator
		const evmContractChange = EVMContractChangeNone
		const burnerContractChange = BurnerContractChangeDeploy

		migrations := NewCadence1Migrations(
			logger,
			t.TempDir(),
			rwf,
			Options{
				NWorker:              nWorker,
				ChainID:              chainID,
				EVMContractChange:    evmContractChange,
				BurnerContractChange: burnerContractChange,
				VerboseErrorOutput:   true,
				ReportMetrics:        true,

				// Important: 'Test' contract is NOT staged intentionally.
				// So values belongs to types from 'Test' contract should be
				// identified as un-migrated values.
			},
		)

		for _, migration := range migrations {
			err = migration.Migrate(registersByAccount)
			require.NoError(
				t,
				err,
				"migration `%s` failed, logs: %v",
				migration.Name,
				logWriter.logs,
			)
		}

		require.NoError(t, err)

		reportWriter := rwf.reportWriters["metrics-collecting-migration"]
		require.Len(t, reportWriter.entries, 1)

		entry := reportWriter.entries[0]
		require.IsType(t, Metrics{}, entry)

		require.Equal(
			t,
			Metrics{
				TotalValues: 752,
				TotalErrors: 6,
				ErrorsPerContract: map[string]int{
					"A.01cf0e2f2f715450.Test": 6,
				},
				ValuesPerContract: map[string]int{
					"A.01cf0e2f2f715450.Test":               6,
					"A.0ae53cb6e3f42a79.FlowToken":          20,
					"A.f8d6e0586b0a20c7.FlowClusterQC":      6,
					"A.f8d6e0586b0a20c7.FlowDKG":            4,
					"A.f8d6e0586b0a20c7.FlowEpoch":          1,
					"A.f8d6e0586b0a20c7.FlowIDTableStaking": 5,
					"A.f8d6e0586b0a20c7.LockedTokens":       3,
					"A.f8d6e0586b0a20c7.NodeVersionBeacon":  1,
				},
			},
			entry,
		)
	})

	t.Run("staged contract with errors", func(t *testing.T) {

		t.Parallel()

		address, err := common.HexToAddress(testAccountAddress)
		require.NoError(t, err)

		// Get the old payloads
		payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
		require.NoError(t, err)

		registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
		require.NoError(t, err)

		rwf := &testReportWriterFactory{}

		logWriter := &writer{}
		logger := zerolog.New(logWriter).Level(zerolog.InfoLevel)

		const nWorker = 2

		const chainID = flow.Emulator
		const evmContractChange = EVMContractChangeNone
		const burnerContractChange = BurnerContractChangeDeploy

		stagedContracts := []StagedContract{
			{
				Contract: Contract{
					Name: "Test",
					Code: []byte(`access(all) contract Test {}`),
				},
				Address: address,
			},
		}

		migrations := NewCadence1Migrations(
			logger,
			t.TempDir(),
			rwf,
			Options{
				NWorker:              nWorker,
				ChainID:              chainID,
				EVMContractChange:    evmContractChange,
				BurnerContractChange: burnerContractChange,
				VerboseErrorOutput:   true,
				ReportMetrics:        true,
				StagedContracts:      stagedContracts,
			},
		)

		for _, migration := range migrations {
			err = migration.Migrate(registersByAccount)
			require.NoError(
				t,
				err,
				"migration `%s` failed, logs: %v",
				migration.Name,
				logWriter.logs,
			)
		}

		require.NoError(t, err)

		reportWriter := rwf.reportWriters["metrics-collecting-migration"]
		require.Len(t, reportWriter.entries, 1)

		entry := reportWriter.entries[0]
		require.IsType(t, Metrics{}, entry)

		require.Equal(
			t,
			Metrics{
				TotalValues: 752,
				TotalErrors: 6,
				ErrorsPerContract: map[string]int{
					"A.01cf0e2f2f715450.Test": 6,
				},
				ValuesPerContract: map[string]int{
					"A.01cf0e2f2f715450.Test":               6,
					"A.0ae53cb6e3f42a79.FlowToken":          20,
					"A.f8d6e0586b0a20c7.FlowClusterQC":      6,
					"A.f8d6e0586b0a20c7.FlowDKG":            4,
					"A.f8d6e0586b0a20c7.FlowEpoch":          1,
					"A.f8d6e0586b0a20c7.FlowIDTableStaking": 5,
					"A.f8d6e0586b0a20c7.LockedTokens":       3,
					"A.f8d6e0586b0a20c7.NodeVersionBeacon":  1,
				},
			},
			entry,
		)
	})
}
