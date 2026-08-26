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
	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
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

// StorehouseBootstrapMode controls which checkpoint the register store is seeded from
// when it is first created (i.e., when the register Pebble DB is not yet bootstrapped).
type StorehouseBootstrapMode string

const (
	// StorehouseBootstrapModeRootCheckpoint bootstraps the register store from the
	// node's root checkpoint (root.checkpoint in the trie directory).  This is the
	// default mode and corresponds to the historical bootstrap path where every EN
	// starts from the network's genesis/spork root.
	StorehouseBootstrapModeRootCheckpoint StorehouseBootstrapMode = "root-checkpoint"

	// StorehouseBootstrapModeSealedCheckpoint bootstraps the register store from the
	// latest numbered checkpoint in the trie directory (e.g. checkpoint.00000042),
	// which is produced by the compact-execution-state utility.  Use this mode when
	// you want to start an EN from a compacted sealed state rather than replaying the
	// full WAL history from the root checkpoint.
	StorehouseBootstrapModeSealedCheckpoint StorehouseBootstrapMode = "sealed-checkpoint"
)

// CheckpointSource resolves the checkpoint file path, the block height that the
// checkpoint corresponds to, and the trie root hash encoded in the checkpoint.
// It is called only when the register store is not yet bootstrapped.
//
// No error returns are expected during normal operation.
type CheckpointSource func(
	log zerolog.Logger,
	state protocol.State,
	triedir string,
) (checkpointFile string, height uint64, rootHash ledger.RootHash, err error)

// RootCheckpointSource is a [CheckpointSource] that resolves the node's root
// checkpoint (root.checkpoint in triedir) seeded with the height and root hash
// declared in the node's root seal.
//
// No error returns are expected during normal operation.
func RootCheckpointSource(
	log zerolog.Logger,
	state protocol.State,
	triedir string,
) (string, uint64, ledger.RootHash, error) {
	sealedRoot := state.Params().SealedRoot()
	rootSeal := state.Params().Seal()

	if sealedRoot.ID() != rootSeal.BlockID {
		return "", 0, ledger.RootHash{}, fmt.Errorf(
			"mismatching root seal and sealed root: %v != %v", sealedRoot.ID(), rootSeal.BlockID)
	}

	checkpointFile := path.Join(triedir, modelbootstrap.FilenameWALRootCheckpoint)
	return checkpointFile, sealedRoot.Height, ledger.RootHash(rootSeal.FinalState), nil
}

// SealedCheckpointSource is a [CheckpointSource] that resolves the latest
// numbered checkpoint produced by the compact-execution-state utility.  It reads
// the checkpoint to extract the single trie's root hash, and derives the
// corresponding block height from the current sealed block in the protocol state.
//
// No error returns are expected during normal operation.
func SealedCheckpointSource(
	log zerolog.Logger,
	state protocol.State,
	triedir string,
) (string, uint64, ledger.RootHash, error) {
	checkpointNums, _, err := wal.ListCheckpoints(triedir)
	if err != nil {
		return "", 0, ledger.RootHash{}, fmt.Errorf("cannot list checkpoints in %s: %w", triedir, err)
	}
	if len(checkpointNums) == 0 {
		return "", 0, ledger.RootHash{}, fmt.Errorf(
			"no checkpoint found in %s; run compact-execution-state first", triedir)
	}

	latestNum := checkpointNums[0]
	for _, n := range checkpointNums[1:] {
		if n > latestNum {
			latestNum = n
		}
	}
	latestName := wal.NumberToFilename(latestNum)

	tries, err := wal.OpenAndReadCheckpointV6(triedir, latestName, log)
	if err != nil {
		return "", 0, ledger.RootHash{}, fmt.Errorf("cannot read checkpoint %s: %w", latestName, err)
	}
	if len(tries) != 1 {
		return "", 0, ledger.RootHash{}, fmt.Errorf(
			"sealed checkpoint %s must contain exactly 1 trie, found %d", latestName, len(tries))
	}

	rootHash := ledger.RootHash(tries[0].RootHash())

	sealedHead, err := state.Sealed().Head()
	if err != nil {
		return "", 0, ledger.RootHash{}, fmt.Errorf("cannot get sealed head: %w", err)
	}

	checkpointFile := path.Join(triedir, latestName)
	log.Info().
		Str("checkpoint", latestName).
		Uint64("height", sealedHead.Height).
		Str("root-hash", rootHash.String()).
		Msg("resolved sealed checkpoint source")

	return checkpointFile, sealedHead.Height, rootHash, nil
}

// CheckpointSourceForMode returns the [CheckpointSource] corresponding to the given
// [StorehouseBootstrapMode].  An unrecognized mode falls back to [RootCheckpointSource].
func CheckpointSourceForMode(mode StorehouseBootstrapMode) CheckpointSource {
	if mode == StorehouseBootstrapModeSealedCheckpoint {
		return SealedCheckpointSource
	}
	return RootCheckpointSource
}

// LoadRegisterStore creates and initializes a RegisterStore.
// It handles opening the pebble database, bootstrapping if needed, and creating the RegisterStore.
// When the database is not yet bootstrapped checkpointSource is called to determine which
// checkpoint file, block height, and root hash to import; use [RootCheckpointSource] for
// the default behaviour or [SealedCheckpointSource] for compacted-state bootstrapping.
func LoadRegisterStore(
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
	checkpointSource CheckpointSource,
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
		originalErr := fmt.Errorf("could not check if registers db is bootstrapped: %w", err)
		return nil, nil, multierror.Append(originalErr, closer.Close()).ErrorOrNil()
	}

	log.Info().Msgf("register store bootstrapped: %v", bootstrapped)

	if !bootstrapped {
		checkpointFile, checkpointHeight, rootHash, err := checkpointSource(log, state, triedir)
		if err != nil {
			originalErr := fmt.Errorf("could not resolve checkpoint source: %w", err)
			return nil, nil, multierror.Append(originalErr, closer.Close()).ErrorOrNil()
		}

		err = importFunc(log.With().Str("component", "background-indexing").Logger(),
			checkpointFile, checkpointHeight, rootHash, pebbledb, importCheckpointWorkerCount)
		if err != nil {
			originalErr := fmt.Errorf("could not import registers from checkpoint: %w", err)
			return nil, nil, multierror.Append(originalErr, closer.Close()).ErrorOrNil()
		}
	}

	diskStore, err := storagepebble.NewRegisters(pebbledb, storagepebble.PruningDisabled)
	if err != nil {
		originalErr := fmt.Errorf("could not create registers storage: %w", err)
		return nil, nil, multierror.Append(originalErr, closer.Close()).ErrorOrNil()
	}

	reader := finalizedreader.NewFinalizedReader(headers, lastFinalizedHeight)
	protocolEvents.AddConsumer(reader)
	notifier := NewRegisterStoreMetrics(collector)

	// report latest finalized and executed height as metrics
	notifier.OnFinalizedAndExecutedHeightUpdated(diskStore.LatestHeight())

	registerStore, err := NewRegisterStore(
		diskStore,
		nil, // TODO(leo): replace with real WAL in storehouse phase 4
		reader,
		log,
		notifier,
	)
	if err != nil {
		return nil, nil, multierror.Append(err, closer.Close()).ErrorOrNil()
	}

	return registerStore, closer, nil
}

// LoadBackgroundIndexerEngine creates and initializes a BackgroundIndexerEngine.
func LoadBackgroundIndexerEngine(
	log zerolog.Logger,
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
	checkpointSource CheckpointSource,
	executionDataStore execution_data.ExecutionDataGetter,
	resultsReader storageerr.ExecutionResultsReader,
	blockExecutedNotifier BlockExecutedNotifier, // optional: notifier for block executed events
	followerDistributor *pubsub.FollowerDistributor,
	heightsPerSecond uint64, // rate limit for indexing heights per second
) (*BackgroundIndexerEngine, bool, error) {

	lg := log.With().Str("component", "background_indexer_loader").Logger()

	if !enableBackgroundStorehouseIndexing {
		lg.Info().Msg("background indexer engine disabled, since --enable-background-storehouse-indexing==false")
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
		registerStore, closer, err := LoadRegisterStore(
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
			checkpointSource,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load register store: %w", err)
		}

		// Create the register updates provider
		provider := NewExecutionDataRegisterUpdatesProvider(
			executionDataStore,
			resultsReader,
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
