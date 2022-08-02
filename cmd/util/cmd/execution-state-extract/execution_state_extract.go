package extract

import (
	"fmt"
	"math"

	"github.com/rs/zerolog"

	mgr "github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func extractExecutionState(
	dir string,
	targetHash flow.StateCommitment,
	outputDir string,
	log zerolog.Logger,
	chain flow.Chain,
	migrate bool,
	report bool,
) error {

	diskWal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return fmt.Errorf("cannot create disk WAL: %w", err)
	}

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)
	compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), complete.DefaultCacheSize, checkpointDistance, checkpointsToKeep)
	if err != nil {
		return fmt.Errorf("cannot create compactor: %w", err)
	}

	<-compactor.Ready()

	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	var migrations []ledger.Migration
	var preCheckpointReporters, postCheckpointReporters []ledger.Reporter
	newState := ledger.State(targetHash)

	if migrate {
		storageUsedUpdateMigration := mgr.StorageUsedUpdateMigration{
			Log:       log,
			OutputDir: outputDir,
		}
		accountStatusMigration := mgr.NewAccountStatusMigration(log)
		legacyControllerMigration := mgr.LegacyControllerMigration{Logger: log}

		migrations = []ledger.Migration{
			accountStatusMigration.Migrate,
			legacyControllerMigration.Migrate,
			storageUsedUpdateMigration.Migrate,
			mgr.PruneMigration,
		}

	}
	// generating reports at the end, so that the checkpoint file can be used
	// for sporking as soon as it's generated.
	if report {
		log.Info().Msgf("preparing reporter files")
		reportFileWriterFactory := reporters.NewReportFileWriterFactory(outputDir, log)

		preCheckpointReporters = []ledger.Reporter{
			// report epoch counter which is needed for finalizing root block
			reporters.NewExportReporter(log,
				chain,
				func() flow.StateCommitment { return targetHash },
			),
		}

		postCheckpointReporters = []ledger.Reporter{
			&reporters.AccountReporter{
				Log:   log,
				Chain: chain,
				RWF:   reportFileWriterFactory,
			},
			reporters.NewFungibleTokenTracker(log, reportFileWriterFactory, chain, []string{reporters.FlowTokenTypeID(chain)}),
			&reporters.AtreeReporter{
				Log: log,
				RWF: reportFileWriterFactory,
			},
		}
	}

	migratedState, err := led.ExportCheckpointAt(
		newState,
		migrations,
		preCheckpointReporters,
		postCheckpointReporters,
		complete.DefaultPathFinderVersion,
		outputDir,
		bootstrap.FilenameWALRootCheckpoint,
	)
	if err != nil {
		return fmt.Errorf("cannot generate the output checkpoint: %w", err)
	}

	log.Info().Msgf(
		"New state commitment for the exported state is: %s (base64: %s)",
		migratedState.String(),
		migratedState.Base64(),
	)

	return nil
}
