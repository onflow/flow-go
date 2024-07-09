package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	syncAtomic "sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/cadence/runtime/common"

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
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

type extractor interface {
	extract() (partialState bool, payloads []*ledger.Payload, err error)
}

type payloadFileExtractor struct {
	logger   zerolog.Logger
	fileName string
}

func newPayloadFileExtractor(
	logger zerolog.Logger,
	fileName string,
) *payloadFileExtractor {
	return &payloadFileExtractor{
		logger:   logger,
		fileName: fileName,
	}
}

func (e *payloadFileExtractor) extract() (bool, []*ledger.Payload, error) {
	return util.ReadPayloadFile(e.logger, e.fileName)
}

type executionStateExtractor struct {
	logger          zerolog.Logger
	dir             string
	stateCommitment flow.StateCommitment
}

func newExecutionStateExtractor(
	logger zerolog.Logger,
	executionStateDir string,
	stateCommitment flow.StateCommitment,
) *executionStateExtractor {
	return &executionStateExtractor{
		logger:          logger,
		dir:             executionStateDir,
		stateCommitment: stateCommitment,
	}
}

func (e *executionStateExtractor) extract() (bool, []*ledger.Payload, error) {
	payloads, err := util.ReadTrie(e.dir, e.stateCommitment)
	if err != nil {
		return false, nil, err
	}

	return false, payloads, nil
}

type exporter interface {
	export(partialState bool, payloads []*ledger.Payload) (ledger.State, error)
}

type payloadFileExporter struct {
	logger         zerolog.Logger
	nWorker        int
	fileName       string
	addressFilters []common.Address
}

func newPayloadFileExporter(
	logger zerolog.Logger,
	nWorker int,
	fileName string,
	addressFilters []common.Address,
) *payloadFileExporter {
	return &payloadFileExporter{
		logger:         logger,
		nWorker:        nWorker,
		fileName:       fileName,
		addressFilters: addressFilters,
	}
}

func (e *payloadFileExporter) export(
	partialState bool,
	payloads []*ledger.Payload,
) (ledger.State, error) {

	var group errgroup.Group

	var migratedState ledger.State

	// Need to use a copy of payloads when creating new trie in goroutine
	// because payloads are sorted in createPayloadFile().
	copiedPayloads := make([]*ledger.Payload, len(payloads))
	copy(copiedPayloads, payloads)

	// Launch goroutine to get root hash of trie from exported payloads
	group.Go(func() error {
		newTrie, err := createTrieFromPayloads(log.Logger, copiedPayloads)
		if err != nil {
			return err
		}

		migratedState = ledger.State(newTrie.RootHash())
		return nil
	})

	// Export payloads to payload file
	err := e.createPayloadFile(partialState, payloads)
	if err != nil {
		return ledger.DummyState, err
	}

	err = group.Wait()
	if err != nil {
		return ledger.DummyState, err
	}

	return migratedState, nil
}

func (e *payloadFileExporter) createPayloadFile(
	partialState bool,
	payloads []*ledger.Payload,
) error {
	e.logger.Info().Msgf("sorting %d payloads", len(payloads))

	// Sort payloads to produce deterministic payload file with
	// same sequence of payloads inside.
	payloads = util.SortPayloadsByAddress(payloads, e.nWorker)

	log.Info().Msgf("sorted %d payloads", len(payloads))

	log.Info().Msgf("creating payloads file %s", e.fileName)

	exportedPayloadCount, err := util.CreatePayloadFile(
		e.logger,
		e.fileName,
		payloads,
		e.addressFilters,
		partialState,
	)
	if err != nil {
		return fmt.Errorf("cannot generate payloads file: %w", err)
	}

	e.logger.Info().Msgf("exported %d payloads out of %d payloads", exportedPayloadCount, len(payloads))

	return nil
}

type checkpointFileExporter struct {
	logger    zerolog.Logger
	outputDir string
}

func newCheckpointFileExporter(
	logger zerolog.Logger,
	outputDir string,
) *checkpointFileExporter {
	return &checkpointFileExporter{
		logger:    logger,
		outputDir: outputDir,
	}
}

func (e *checkpointFileExporter) export(
	_ bool,
	payloads []*ledger.Payload,
) (ledger.State, error) {
	// Create trie
	newTrie, err := createTrieFromPayloads(e.logger, payloads)
	if err != nil {
		return ledger.DummyState, err
	}

	// Create checkpoint files
	return createCheckpoint(
		log.Logger,
		newTrie,
		e.outputDir,
		bootstrap.FilenameWALRootCheckpoint,
	)
}

func createCheckpoint(
	log zerolog.Logger,
	newTrie *trie.MTrie,
	outputDir,
	outputFile string,
) (ledger.State, error) {
	statecommitment := ledger.State(newTrie.RootHash())

	log.Info().Msgf("successfully built new trie. NEW ROOT STATECOMMIEMENT: %v", statecommitment.String())

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
	return statecommitment, nil
}

func writeStatusFile(fileName string, e error) error {
	checkpointStatus := map[string]bool{"succeeded": e == nil}
	checkpointStatusJson, _ := json.MarshalIndent(checkpointStatus, "", " ")
	err := os.WriteFile(fileName, checkpointStatusJson, 0644)
	return err
}

func createMigration(
	log zerolog.Logger,
	outputDir string,
	nWorker int,
	fixSlabsWithBrokenReferences bool,
) []ledger.Migration {
	migrations := newMigrations(
		log,
		outputDir,
		fixSlabsWithBrokenReferences,
	)

	migration := newMigration(log, migrations, nWorker)

	return []ledger.Migration{migration}
}

func migratePayloads(logger zerolog.Logger, payloads []*ledger.Payload, migrations []ledger.Migration) ([]*ledger.Payload, error) {

	if len(migrations) == 0 {
		return payloads, nil
	}

	var err error
	payloadCount := len(payloads)

	// migrate payloads
	for i, migrate := range migrations {
		logger.Info().Msgf("migration %d/%d is underway", i, len(migrations))

		start := time.Now()
		payloads, err = migrate(payloads)
		elapsed := time.Since(start)

		if err != nil {
			return nil, fmt.Errorf("error applying migration (%d): %w", i, err)
		}

		newPayloadCount := len(payloads)

		if payloadCount != newPayloadCount {
			logger.Warn().
				Int("migration_step", i).
				Int("expected_size", payloadCount).
				Int("outcome_size", newPayloadCount).
				Msg("payload counts has changed during migration, make sure this is expected.")
		}
		logger.Info().Str("timeTaken", elapsed.String()).Msgf("migration %d is done", i)

		payloadCount = newPayloadCount
	}

	return payloads, nil
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
	applyPruning := false
	newTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, derefPayloads, applyPruning)
	if err != nil {
		return nil, fmt.Errorf("constructing updated trie failed: %w", err)
	}

	return newTrie, nil
}

func newMigrations(
	log zerolog.Logger,
	outputDir string,
	fixSlabsWithBrokenReferences bool,
) []migrators.AccountBasedMigration {

	rwf := reporters.NewReportFileWriterFactory(outputDir, log)

	var accountBasedMigrations []migrators.AccountBasedMigration

	if fixSlabsWithBrokenReferences {
		accountBasedMigrations = append(
			accountBasedMigrations,
			migrators.NewFixBrokenReferencesInSlabsMigration(
				outputDir,
				rwf,
				migrators.TestnetAccountsWithBrokenSlabReferences,
			),
		)
	}

	if flagFilterUnreferencedSlabs {
		accountBasedMigrations = append(
			accountBasedMigrations,
			migrators.NewFilterUnreferencedSlabsMigration(
				outputDir,
				rwf,
			),
		)
	}

	accountBasedMigrations = append(
		accountBasedMigrations,
		migrators.NewAtreeRegisterMigrator(
			rwf,
			flagValidateMigration,
			flagLogVerboseValidationError,
			flagContinueMigrationOnValidationError,
			flagCheckStorageHealthBeforeMigration,
			flagCheckStorageHealthAfterMigration,
		),

		&migrators.DeduplicateContractNamesMigration{},

		// This will fix storage used discrepancies caused by the previous migrations
		&migrators.AccountUsageMigration{},
	)

	return accountBasedMigrations
}

func newMigration(
	logger zerolog.Logger,
	migrations []migrators.AccountBasedMigration,
	nWorker int,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {
		if len(migrations) == 0 {
			return payloads, nil
		}

		payloadCount := len(payloads)

		logger.Info().Msgf(
			"grouping %d payloads ...",
			payloadCount,
		)

		payloadAccountGrouping := util.GroupPayloadsByAccount(logger, payloads, nWorker)

		logger.Info().Msgf(
			"creating registers from grouped payloads (%d groups) ...",
			payloadAccountGrouping.Len(),
		)

		registersByAccount, err := newByAccountRegistersFromPayloadAccountGrouping(payloadAccountGrouping, nWorker)
		if err != nil {
			return nil, err
		}

		logger.Info().Msgf(
			"created registers from %d payloads (%d accounts)",
			payloadCount,
			registersByAccount.AccountCount(),
		)

		err = migrators.MigrateByAccount(
			logger,
			nWorker,
			registersByAccount,
			migrations,
		)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to migrate")
			return nil, err
		}

		logger.Info().Msg("creating new payloads from registers ...")

		newPayloads := registersByAccount.DestructIntoPayloads(nWorker)

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

	jobs := make(chan *util.PayloadAccountGroup, nWorker)
	results := make(chan *registers.AccountRegisters, nWorker)

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

				// Convert address to owner
				payloadGroupOwner := flow.AddressToRegisterOwner(payloadAccountGroup.Address)

				accountRegisters, err := registers.NewAccountRegistersFromPayloads(
					payloadGroupOwner,
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
				// Account grouping should never create multiple groups for an account.
				// In case it does anyway, merge the groups together,
				// by merging the existing registers into the new ones.

				log.Warn().Msgf(
					"account registers already exist for account %x. merging %d existing registers into %d new",
					accountRegisters.Owner(),
					oldAccountRegisters.Count(),
					accountRegisters.Count(),
				)

				err := accountRegisters.Merge(oldAccountRegisters)
				if err != nil {
					return fmt.Errorf("failed to merge account registers: %w", err)
				}
			}
		}

		return nil
	})

	return registersByAccount, g.Wait()
}
