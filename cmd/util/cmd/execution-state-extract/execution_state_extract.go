package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime/pprof"
	syncAtomic "sync/atomic"
	"time"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"

	migrators "github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

func extractExecutionState(
	log zerolog.Logger,
	dir string,
	targetHash flow.StateCommitment,
	outputDir string,
	nWorker int, // number of concurrent worker to migration payloads
	runMigrations bool,
	outputPayloadFile string,
	exportPayloadsByAddresses []common.Address,
	sortPayloads bool,
	opts migrators.Options,
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
		&metrics.NoopCollector{},
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

	migrations := newMigrations(
		log,
		dir,
		runMigrations,
		opts,
	)

	migration := newMigration(log, migrations, nWorker)

	newState := ledger.State(targetHash)

	// migrate the trie if there are migrations
	newTrie, err := led.MigrateAt(
		newState,
		migration,
		complete.DefaultPathFinderVersion,
	)

	if err != nil {
		return err
	}

	// create reporter
	reporter := reporters.NewExportReporter(
		log,
		func() flow.StateCommitment {
			return targetHash
		},
	)

	newMigratedState := ledger.State(newTrie.RootHash())
	err = reporter.Report(nil, newMigratedState)
	if err != nil {
		log.Err(err).Msgf("can not generate report for migrated state: %v", newMigratedState)
	}

	if len(outputPayloadFile) > 0 {
		payloads := newTrie.AllPayloads()

		return exportPayloads(
			log,
			payloads,
			nWorker,
			outputPayloadFile,
			exportPayloadsByAddresses,
			false, // payloads represents entire state.
			sortPayloads,
		)
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

func extractExecutionStateFromPayloads(
	log zerolog.Logger,
	dir string,
	outputDir string,
	nWorker int, // number of concurrent worker to migration payloads
	runMigrations bool,
	inputPayloadFile string,
	outputPayloadFile string,
	exportPayloadsByAddresses []common.Address,
	sortPayloads bool,
	opts migrators.Options,
) error {

	inputPayloadsFromPartialState, payloads, err := util.ReadPayloadFile(log, inputPayloadFile)
	if err != nil {
		return err
	}

	log.Info().Msgf("read %d payloads", len(payloads))

	migrations := newMigrations(
		log,
		dir,
		runMigrations,
		opts,
	)

	migration := newMigration(
		log,
		migrations,
		nWorker,
	)

	payloads, err = migration(payloads)
	if err != nil {
		return err
	}

	if len(outputPayloadFile) > 0 {
		return exportPayloads(
			log,
			payloads,
			nWorker,
			outputPayloadFile,
			exportPayloadsByAddresses,
			inputPayloadsFromPartialState,
			sortPayloads,
		)
	}

	newTrie, err := createTrieFromPayloads(log, payloads)
	if err != nil {
		return err
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

func exportPayloads(
	log zerolog.Logger,
	payloads []*ledger.Payload,
	nWorker int,
	outputPayloadFile string,
	exportPayloadsByAddresses []common.Address,
	inputPayloadsFromPartialState bool,
	sortPayloads bool,
) error {
	if sortPayloads {
		log.Info().Msgf("sorting %d payloads", len(payloads))

		// Sort payloads to produce deterministic payload file with
		// same sequence of payloads inside.
		payloads = util.SortPayloadsByAddress(payloads, nWorker)

		log.Info().Msgf("sorted %d payloads", len(payloads))
	}

	log.Info().Msgf("creating payloads file %s", outputPayloadFile)

	exportedPayloadCount, err := util.CreatePayloadFile(
		log,
		outputPayloadFile,
		payloads,
		exportPayloadsByAddresses,
		inputPayloadsFromPartialState,
	)
	if err != nil {
		return fmt.Errorf("cannot generate payloads file: %w", err)
	}

	log.Info().Msgf("exported %d payloads out of %d payloads", exportedPayloadCount, len(payloads))

	return nil
}

func newMigration(
	logger zerolog.Logger,
	migrations []migrators.NamedMigration,
	nWorker int,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
		if flagCPUProfile != "" {
			f, err := os.Create(flagCPUProfile)
			if err != nil {
				logger.Fatal().Err(err).Msg("could not create CPU profile")
			}

			err = pprof.StartCPUProfile(f)
			if err != nil {
				logger.Fatal().Err(err).Msg("could not start CPU profile")
			}

			defer pprof.StopCPUProfile()
		}

		if len(migrations) == 0 {
			return payloads, nil
		}

		payloadCount := len(payloads)

		payloadAccountGrouping := util.GroupPayloadsByAccount(logger, payloads, nWorker)

		logger.Info().Msgf(
			"creating registers from grouped payloads (%d) ...",
			payloadCount,
		)

		registersByAccount, err := newByAccountRegistersFromPayloadAccountGrouping(payloadAccountGrouping, nWorker)
		if err != nil {
			return nil, err
		}

		logger.Info().Msgf(
			"created registers from payloads (%d accounts)",
			registersByAccount.AccountCount(),
		)

		// Run all migrations on the registers
		for index, migration := range migrations {
			migrationStep := index + 1

			logger.Info().
				Str("migration", migration.Name).
				Msgf(
					"migration %d/%d is underway",
					migrationStep,
					len(migrations),
				)

			start := time.Now()
			err := migration.Migrate(registersByAccount)
			elapsed := time.Since(start)
			if err != nil {
				return nil, fmt.Errorf(
					"error applying migration %s (%d/%d): %w",
					migration.Name,
					migrationStep,
					len(migrations),
					err,
				)
			}

			newPayloadCount := registersByAccount.Count()

			if payloadCount != newPayloadCount {
				logger.Warn().
					Int("migration_step", migrationStep).
					Int("expected_size", payloadCount).
					Int("outcome_size", newPayloadCount).
					Msg("payload counts has changed during migration, make sure this is expected.")
			}

			logger.Info().
				Str("timeTaken", elapsed.String()).
				Str("migration", migration.Name).
				Msgf(
					"migration %d/%d is done",
					migrationStep,
					len(migrations),
				)

			payloadCount = newPayloadCount
		}

		logger.Info().Msg("creating new payloads from registers ...")

		newPayloads := registersByAccount.Payloads()

		logger.Info().Msgf("created new payloads (%d) from registers", len(newPayloads))

		return newPayloads, nil
	}
}

func newByAccountRegistersFromPayloadAccountGrouping(
	payloadAccountGrouping *util.PayloadAccountGrouping,
	nWorker int,
) (
	*registers.ByAccount,
	error,
) {
	g, ctx := errgroup.WithContext(context.Background())

	jobs := make(chan *util.PayloadAccountGroup)
	results := make(chan *registers.AccountRegisters)

	g.Go(func() error {
		defer close(jobs)
		for {
			payloadAccountGroup, err := payloadAccountGrouping.Next()
			if err != nil {
				return fmt.Errorf("failed to group payloads by account: %w", err)
			}

			if payloadAccountGroup == nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- payloadAccountGroup:
			}
		}
	})

	workersLeft := int64(nWorker)
	for i := 0; i < nWorker; i++ {
		g.Go(func() error {
			defer func() {
				if syncAtomic.AddInt64(&workersLeft, -1) == 0 {
					close(results)
				}
			}()

			for payloadAccountGroup := range jobs {
				accountRegisters, err := registers.NewAccountRegistersFromPayloads(
					string(payloadAccountGroup.Address[:]),
					payloadAccountGroup.Payloads,
				)
				if err != nil {
					return fmt.Errorf("failed to create account registers from payloads: %w", err)
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- accountRegisters:
				}
			}

			return nil
		})
	}

	registersByAccount := registers.NewByAccount()
	g.Go(func() error {
		for accountRegisters := range results {
			oldAccountRegisters := registersByAccount.SetAccountRegisters(accountRegisters)
			if oldAccountRegisters != nil {
				return fmt.Errorf(
					"duplicate account registers for account %s",
					accountRegisters.Owner(),
				)
			}
		}

		return nil
	})

	return registersByAccount, g.Wait()
}

func createTrieFromPayloads(logger zerolog.Logger, payloads []*ledger.Payload) (*trie.MTrie, error) {
	// get paths
	paths, err := pathfinder.PathsFromPayloads(payloads, complete.DefaultPathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot export checkpoint, can't construct paths: %w", err)
	}

	logger.Info().Msgf("constructing a new trie with migrated payloads (count: %d)...", len(payloads))

	emptyTrie := trie.NewEmptyMTrie()

	derefPayloads := make([]ledger.Payload, len(payloads))
	for i, p := range payloads {
		derefPayloads[i] = *p
	}

	// no need to prune the data since it has already been prunned through migrations
	const applyPruning = false
	newTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, derefPayloads, applyPruning)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	return newTrie, nil
}

func newMigrations(
	log zerolog.Logger,
	outputDir string,
	runMigrations bool,
	opts migrators.Options,
) []migrators.NamedMigration {
	if !runMigrations {
		return nil
	}

	log.Info().Msg("initializing migrations")

	rwf := reporters.NewReportFileWriterFactory(outputDir, log)

	namedMigrations := migrators.NewCadence1Migrations(
		log,
		outputDir,
		rwf,
		opts,
	)

	// At the end, fix up storage-used discrepancies
	namedMigrations = append(
		namedMigrations,
		migrators.NamedMigration{
			Name: "account-usage-migration",
			Migrate: migrators.NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]migrators.AccountBasedMigration{
					&migrators.AccountUsageMigration{},
				},
			),
		},
	)

	log.Info().Msg("initialized migrations")

	return namedMigrations
}
