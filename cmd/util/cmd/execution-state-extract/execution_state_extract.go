package extract

import (
	"fmt"

	"github.com/rs/zerolog"

	mgr "github.com/onflow/flow-go/cmd/util/ledger/migrations"
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

func extractExecutionState(dir string,
	targetHash flow.StateCommitment,
	outputDir string,
	log zerolog.Logger,
	migrate bool,
	report bool) error {

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
	defer func() {
		<-diskWal.Done()
	}()

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	migrations := []ledger.Migration{}
	reporters := []ledger.Reporter{}

	storageUsedUpdateMigration := mgr.StorageUsedUpdateMigration{Log: log, OutputDir: outputDir}

	if migrate {
		migrations = []ledger.Migration{
			mgr.PruneMigration,
			storageUsedUpdateMigration.Migrate,
		}
	}
	if report {
		reporters = []ledger.Reporter{
			mgr.ContractReporter{Log: log, OutputDir: outputDir},
			mgr.StorageReporter{Log: log, OutputDir: outputDir},
		}
	}
	newState, err := led.ExportCheckpointAt(
		ledger.State(targetHash),
		migrations,
		reporters,
		complete.DefaultPathFinderVersion,
		outputDir,
		bootstrap.FilenameWALRootCheckpoint,
	)
	if err != nil {
		return fmt.Errorf("cannot generate the output checkpoint: %w", err)
	}

	log.Info().Msgf(
		"New state commitment for the exported state is: %s (base64: %s)",
		newState.String(),
		newState.Base64(),
	)

	return nil
}
