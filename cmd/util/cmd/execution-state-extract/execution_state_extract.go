package extract

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	migrators "github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
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
	log zerolog.Logger,
	dir string,
	targetHash flow.StateCommitment,
	outputDir string,
	nWorker int, // number of concurrent worker to migration payloads
	runMigrations bool,
) error {

	log.Info().Msg("init WAL")

	diskWal, err := wal.NewDiskWAL(
		log,
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

	log.Info().Msg("init ledger")

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

	log.Info().Msg("init compactor")

	compactor, err := complete.NewCompactor(
		led,
		diskWal,
		log,
		complete.DefaultCacheSize,
		checkpointDistance,
		checkpointsToKeep,
		atomic.NewBool(false),
	)
	if err != nil {
		return fmt.Errorf("cannot create compactor: %w", err)
	}

	log.Info().Msgf("waiting for compactor to load checkpoint and WAL")

	<-compactor.Ready()

	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	var migrations []ledger.Migration

	if runMigrations {
		rwf := reporters.NewReportFileWriterFactory(dir, log)

		migrations = []ledger.Migration{
			// Contracts must be migrated first
			migrators.CreateAccountBasedMigration(
				log,
				nWorker,
				[]migrators.AccountBasedMigration{
					migrators.NewStagedContractsMigration(migrators.GetStagedContracts),
				},
			),

			migrators.CreateAccountBasedMigration(
				log,
				nWorker,
				migrators.NewCadenceMigrations(rwf),
			),
		}
	}

	newState := ledger.State(targetHash)

	// migrate the trie if there are migrations
	newTrie, err := led.MigrateAt(
		newState,
		migrations,
		complete.DefaultPathFinderVersion,
	)

	if err != nil {
		return err
	}

	// create reporter
	reporter := reporters.NewExportReporter(
		log,
		func() flow.StateCommitment { return targetHash },
	)

	newMigratedState := ledger.State(newTrie.RootHash())
	err = reporter.Report(nil, newMigratedState)
	if err != nil {
		log.Error().Err(err).Msgf("can not generate report for migrated state: %v", newMigratedState)
	}

	migratedState, err := createCheckpoint(
		newTrie,
		log,
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

func createCheckpoint(
	newTrie *trie.MTrie,
	log zerolog.Logger,
	outputDir,
	outputFile string,
) (ledger.State, error) {
	stateCommitment := ledger.State(newTrie.RootHash())

	log.Info().Msgf("successfully built new trie. NEW ROOT STATECOMMIEMENT: %v", stateCommitment.String())

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("could not create output dir %s: %w", outputDir, err)
	}

	err = wal.StoreCheckpointV6Concurrently([]*trie.MTrie{newTrie}, outputDir, outputFile, log)

	// Writing the checkpoint takes time to write and copy.
	// Without relying on an exit code or stdout, we need to know when the copy is complete.
	writeStatusFileErr := writeStatusFile("checkpoint_status.json", err)
	if writeStatusFileErr != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to write checkpoint status file: %w", writeStatusFileErr)
	}

	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to store the checkpoint: %w", err)
	}

	log.Info().Msgf("checkpoint file successfully stored at: %v %v", outputDir, outputFile)
	return stateCommitment, nil
}

func writeStatusFile(fileName string, e error) error {
	checkpointStatus := map[string]bool{"succeeded": e == nil}
	checkpointStatusJson, _ := json.MarshalIndent(checkpointStatus, "", " ")
	err := os.WriteFile(fileName, checkpointStatusJson, 0644)
	return err
}
