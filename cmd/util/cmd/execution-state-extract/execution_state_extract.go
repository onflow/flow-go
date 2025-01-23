package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	migrators "github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

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
	addressFilters map[string]struct{}
	sortPayloads   bool
}

func newPayloadFileExporter(
	logger zerolog.Logger,
	nWorker int,
	fileName string,
	addressFilters map[string]struct{},
	sortPayloads bool,
) *payloadFileExporter {
	return &payloadFileExporter{
		logger:         logger,
		nWorker:        nWorker,
		fileName:       fileName,
		addressFilters: addressFilters,
		sortPayloads:   sortPayloads,
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
	if e.sortPayloads {
		e.logger.Info().Msgf("sorting %d payloads", len(payloads))

		// Sort payloads to produce deterministic payload file with
		// same sequence of payloads inside.
		payloads = util.SortPayloadsByAddress(payloads, e.nWorker)

		log.Info().Msgf("sorted %d payloads", len(payloads))
	}

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

func newMigration(
	logger zerolog.Logger,
	migrations []migrators.NamedMigration,
	nWorker int,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		if len(migrations) == 0 {
			return payloads, nil
		}

		payloadCount := len(payloads)

		payloadAccountGrouping := util.GroupPayloadsByAccount(logger, payloads, nWorker)

		logger.Info().Msgf(
			"creating registers from grouped payloads (%d) ...",
			payloadCount,
		)

		registersByAccount, err := util.NewByAccountRegistersFromPayloadAccountGrouping(payloadAccountGrouping, nWorker)
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

		newPayloads := registersByAccount.DestructIntoPayloads(nWorker)

		logger.Info().Msgf("created new payloads (%d) from registers", len(newPayloads))

		return newPayloads, nil
	}
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
