package storehouse

import (
	"fmt"
	"io"
	"path"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
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

	log.Info().
		Str("pebble_db_path", registerDir).
		Msg("register store enabled")
	pebbledb, err := storagepebble.OpenRegisterPebbleDB(
		log.With().Str("pebbledb", "registers").Logger(),
		registerDir)

	if err != nil {
		return nil, nil, fmt.Errorf("could not create disk register store: %w", err)
	}

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

		err = importFunc(log, checkpointFile, checkpointHeight, rootHash, pebbledb, importCheckpointWorkerCount)
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
// It returns nil if background indexing is disabled or if storehouse is enabled.
func LoadBackgroundIndexerEngine(
	log zerolog.Logger,
	enableStorehouse bool,
	enableBackgroundStorehouseIndexing bool,
	registerStore execution.RegisterStore,
	executionDataStore execution_data.ExecutionDataGetter,
	resultsReader storageerr.ExecutionResultsReader,
	state protocol.State,
	headers storageerr.Headers,
) (*BackgroundIndexerEngine, error) {
	// Only create background indexer engine if storehouse is not enabled
	// and background indexing is enabled
	if enableStorehouse {
		log.Info().Msg("background indexer engine disabled (storehouse enabled)")
		return nil, nil
	}

	if !enableBackgroundStorehouseIndexing {
		log.Info().Msg("background indexer engine disabled")
		return nil, nil
	}

	// Check that required dependencies are available
	if registerStore == nil {
		return nil, fmt.Errorf("register store is not initialized")
	}
	if executionDataStore == nil {
		return nil, fmt.Errorf("execution data store is not initialized")
	}
	if resultsReader == nil {
		return nil, fmt.Errorf("execution results reader is not initialized")
	}

	// Create the register updates provider
	provider := NewExecutionDataRegisterUpdatesProvider(
		executionDataStore,
		resultsReader,
		headers,
	)

	// Create the background indexer
	backgroundIndexer := NewBackgroundIndexer(
		provider,
		registerStore,
		state,
		headers,
	)

	// Create the background indexer engine
	backgroundIndexerEngine := NewBackgroundIndexerEngine(
		log,
		backgroundIndexer,
	)

	return backgroundIndexerEngine, nil
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
