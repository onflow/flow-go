// The factory provides functions for the execution_builder to load and initialize
// the register store and background indexer engine, simplifying the builder by
// encapsulating database setup, bootstrapping, and checkpoint import logic.
package storehouse

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/ledger"
	modelbootstrap "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/finalizedreader"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	storageerr "github.com/onflow/flow-go/storage"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
)

// BlockExecutedNotifier is an interface for components that can register callbacks
// to be notified when blocks are executed.
type BlockExecutedNotifier interface {
	AddConsumer(callback func())
}

// ImportRegistersFromCheckpoint imports registers from a checkpoint file.
// It is defined as a function type to avoid a circular dependency; the
// implementation (bootstrap.ImportRegistersFromCheckpoint) is provided by the caller.
type ImportRegistersFromCheckpoint func(logger zerolog.Logger, checkpointFile string, checkpointHeight uint64, checkpointRootHash ledger.RootHash, pdb *pebble.DB, workerCount int) error

// LoadRegisterStore creates and initializes a RegisterStore.
// It handles opening the pebble database, bootstrapping if needed, and creating the RegisterStore.
func LoadRegisterStore(
	log zerolog.Logger,
	state protocol.State,
	headers storageerr.Headers,
	protocolEvents *events.Distributor,
	lastFinalizedHeight uint64,
	collector module.ExecutionMetrics,
	enableStorehouse bool,
	registerDir string,
	triedir string,
	importCheckpointWorkerCount int,
	importFunc ImportRegistersFromCheckpoint,
) (
	*RegisterStore,
	io.Closer,
	error,
) {
	if !enableStorehouse {
		log.Info().Msg("register store disabled")
		return nil, nil, nil
	}
	return loadRegisterStore(
		log,
		state,
		headers,
		protocolEvents,
		lastFinalizedHeight,
		collector,
		registerDir,
		triedir,
		importCheckpointWorkerCount,
		importFunc,
	)
}

// loadRegisterStore is an internal function that creates and initializes a RegisterStore.
// it is reused by both LoadRegisterStore and LoadBackgroundIndexerEngine.
func loadRegisterStore(
	log zerolog.Logger,
	state protocol.State,
	headers storageerr.Headers,
	protocolEvents *events.Distributor,
	lastFinalizedHeight uint64,
	collector module.ExecutionMetrics,
	registerDir string,
	triedir string,
	importCheckpointWorkerCount int,
	importFunc ImportRegistersFromCheckpoint,
) (
	*RegisterStore,
	io.Closer,
	error,
) {
	log.Info().
		Str("pebble_db_path", registerDir).
		Msg("register store enabled")

	pebbledb, err := storagepebble.OpenRegisterPebbleDB(
		log.With().Str("pebbledb", "registers").Logger(),
		registerDir)

	if err != nil {
		return nil, nil, fmt.Errorf("could not create disk register store: %w", err)
	}

	// wrap the pebble db with a struct to include detailed error message
	closer := &pebbleDBCloser{db: pebbledb}

	bootstrapped, err := storagepebble.IsBootstrapped(pebbledb)
	if err != nil {
		return nil, nil, fmt.Errorf("could not check if registers db is bootstrapped: %w", err)
	}

	log.Info().Msgf("register store bootstrapped: %v", bootstrapped)

	if !bootstrapped {
		checkpointFile := path.Join(triedir, modelbootstrap.FilenameWALRootCheckpoint)
		sealedRoot := state.Params().SealedRoot()

		rootSeal := state.Params().Seal()

		if sealedRoot.ID() != rootSeal.BlockID {
			return nil, nil, fmt.Errorf("mismatching root seal and sealed root: %v != %v", sealedRoot.ID(), rootSeal.BlockID)
		}

		checkpointHeight := sealedRoot.Height
		rootHash := ledger.RootHash(rootSeal.FinalState)

		err = importFunc(log.With().Str("component", "background-indexing").Logger(),
			checkpointFile, checkpointHeight, rootHash, pebbledb, importCheckpointWorkerCount)
		if err != nil {
			return nil, nil, fmt.Errorf("could not import registers from checkpoint: %w", err)
		}
	}

	diskStore, err := storagepebble.NewRegisters(pebbledb, storagepebble.PruningDisabled)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create registers storage: %w", err)
	}

	reader := finalizedreader.NewFinalizedReader(headers, lastFinalizedHeight)
	protocolEvents.AddConsumer(reader)
	notifier := NewRegisterStoreMetrics(collector)

	// report latest finalized and executed height as metrics
	notifier.OnFinalizedAndExecutedHeightUpdated(diskStore.LatestHeight())

	registerStore, err := NewRegisterStore(
		diskStore,
		nil, // TODO: replace with real WAL
		reader,
		log,
		notifier,
	)
	if err != nil {
		return nil, nil, err
	}

	return registerStore, closer, nil
}

// LoadBackgroundIndexerEngine creates and initializes a BackgroundIndexerEngine.
func LoadBackgroundIndexerEngine(
	log zerolog.Logger,
	enableStorehouse bool,
	enableBackgroundStorehouseIndexing bool,
	state protocol.State,
	headers storageerr.Headers,
	protocolEvents *events.Distributor,
	lastFinalizedHeight uint64,
	collector module.ExecutionMetrics,
	registerDir string,
	triedir string,
	importCheckpointWorkerCount int,
	importFunc ImportRegistersFromCheckpoint,
	executionDataStore execution_data.ExecutionDataGetter,
	resultsReader storageerr.ExecutionResultsReader,
	blockExecutedNotifier BlockExecutedNotifier, // optional: notifier for block executed events
	followerDistributor *pubsub.FollowerDistributor,
	heightsPerSecond uint64, // rate limit for indexing heights per second
) (*BackgroundIndexerEngine, bool, error) {

	lg := log.With().Str("component", "background_indexer_loader").Logger()

	// Only create background indexer engine if storehouse is not enabled
	// and background indexing is enabled
	if enableStorehouse {
		lg.Info().Msg("background indexer engine disabled, since storehouse is enabled")
		return nil, false, nil
	}

	if !enableBackgroundStorehouseIndexing {
		lg.Info().Msg("background indexer engine disabled, since --enableBackgroundStorehouseIndexing==false")
		return nil, false, nil
	}

	lg.Info().Msg("background indexer engine enabled")

	// Check that required dependencies are available
	if executionDataStore == nil {
		return nil, false, fmt.Errorf("execution data store is not initialized")
	}
	if resultsReader == nil {
		return nil, false, fmt.Errorf("execution results reader is not initialized")
	}

	// bootstrapper function allows deferred initialization of register store
	// and the initial indexing work, so that it happens within the engine's worker loop
	// and not block the component initialization
	bootstrapper := func(ctx context.Context) (*BackgroundIndexer, io.Closer, error) {
		// Load register store for background indexing
		registerStore, closer, err := loadRegisterStore(
			log,
			state,
			headers,
			protocolEvents,
			lastFinalizedHeight,
			collector,
			registerDir,
			triedir,
			importCheckpointWorkerCount,
			importFunc,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load register store: %w", err)
		}

		// Create the register updates provider
		provider := NewExecutionDataRegisterUpdatesProvider(
			executionDataStore,
			resultsReader,
			headers,
		)

		// Create the background indexer
		backgroundIndexer := NewBackgroundIndexer(
			log,
			provider,
			registerStore,
			state,
			headers,
			heightsPerSecond,
		)

		return backgroundIndexer, closer, nil
	}

	// Create the background indexer engine
	backgroundIndexerEngine := NewBackgroundIndexerEngine(
		log,
		bootstrapper,
		blockExecutedNotifier,
		followerDistributor,
	)

	return backgroundIndexerEngine, true, nil
}

type pebbleDBCloser struct {
	db *pebble.DB
}

var _ io.Closer = (*pebbleDBCloser)(nil)

func (c *pebbleDBCloser) Close() error {
	err := c.db.Close()
	if err != nil {
		return fmt.Errorf("could not close register store: %w", err)
	}
	return nil
}
