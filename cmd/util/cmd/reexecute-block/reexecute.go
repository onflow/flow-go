package reexecute_block

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/initialize"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// RegisterGetter reads the value of a register as of a given block height, returning the most recent
// value at or below that height. It is the read side of the Storehouse register store;
// [github.com/onflow/flow-go/storage/pebble.Registers] satisfies it.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: when the register has no value at or below the given height.
//   - [storage.ErrHeightNotIndexed]: when the height is outside the store's indexed range.
type RegisterGetter interface {
	Get(id flow.RegisterID, height uint64) (flow.RegisterValue, error)
}

// StorehouseSnapshotAtHeight returns a read-only [snapshot.StorageSnapshot] that serves register
// values as of the given height from the Storehouse register store. This is an open-world register
// source: it can serve the value of any register, not just a touch-set, so re-execution remains
// valid even when the execution version reads a different set of registers.
//
// A register that does not exist at or below the height reads back as an empty value (nil), matching
// the value FVM expects for an unset register.
//
// The returned snapshot caches reads (see [cachingStorageSnapshot]) so it faithfully mirrors the
// production base snapshot, [storehouse.BlockEndStateSnapshot], which holds a read cache. Because the
// block computer passes one base-snapshot instance to both block execution and proof collection, a
// register read during execution is served from the cache when proof collection re-reads it — exactly
// as in production. Without this cache the benchmark would count a second register-store read at proof
// time that never happens on a real execution node.
func StorehouseSnapshotAtHeight(getter RegisterGetter, height uint64) snapshot.StorageSnapshot {
	return newCachingStorageSnapshot(func(id flow.RegisterID) (flow.RegisterValue, error) {
		value, err := getter.Get(id, height)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// Unset register: FVM expects an empty value, not an error.
				return nil, nil
			}
			return nil, fmt.Errorf("could not read register %s at height %d: %w", id, height, err)
		}
		return value, nil
	})
}

// cachingStorageSnapshot wraps a register read function with an in-memory read cache, mirroring the
// production [storehouse.BlockEndStateSnapshot]: the first read of a register hits the backing store
// and is cached; subsequent reads of the same register (notably proof collection re-reading a register
// already read during block execution) are served from the cache. nil (unset register) values are
// cached too, since the backing store does not change them for a fixed height.
//
// Safe for concurrent access: block execution and proof collection both read concurrently.
type cachingStorageSnapshot struct {
	read  func(flow.RegisterID) (flow.RegisterValue, error)
	mu    sync.RWMutex
	cache map[flow.RegisterID]flow.RegisterValue
}

// newCachingStorageSnapshot returns a caching snapshot over the given read function.
func newCachingStorageSnapshot(
	read func(flow.RegisterID) (flow.RegisterValue, error),
) *cachingStorageSnapshot {
	return &cachingStorageSnapshot{
		read:  read,
		cache: make(map[flow.RegisterID]flow.RegisterValue),
	}
}

// Get returns the register value, serving it from the cache when present and otherwise reading it from
// the backing store and caching the result.
//
// No error returns are expected during normal operation.
func (s *cachingStorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	s.mu.RLock()
	value, ok := s.cache[id]
	s.mu.RUnlock()
	if ok {
		return value, nil
	}

	value, err := s.read(id)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[id] = value
	s.mu.Unlock()
	return value, nil
}

// AssembleExecutableBlock builds an [entity.ExecutableBlock] from a full block, its complete
// collections, and the parent block's final state commitment (the state the block executes against).
//
// It mirrors the assembly performed by the ingestion block queue
// (engine/execution/ingestion/block_queue/queue.go): the complete-collection map is keyed by
// collection ID, each entry pairs the block's guarantee with the corresponding stored collection, and
// StartState is set to the parent state commitment.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: when a collection referenced by the block is not in storage.
func AssembleExecutableBlock(
	block *flow.Block,
	collections storage.Collections,
	parentStateCommitment flow.StateCommitment,
) (*entity.ExecutableBlock, error) {
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(block.Payload.Guarantees))
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not get collection %s: %w", guarantee.CollectionID, err)
		}
		completeCollections[guarantee.CollectionID] = &entity.CompleteCollection{
			Guarantee:  guarantee,
			Collection: collection,
		}
	}

	startState := parentStateCommitment
	return &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
		StartState:          &startState,
	}, nil
}

// NewComputationManager builds a [computation.Manager] configured for persistence-free re-execution.
// The caller supplies the [computer.ViewCommitter], which selects how state commitments and proofs
// are produced:
//   - a no-op committer (see [committer.NewNoopViewCommitter]) requires no trie and produces no state
//     commitment or proofs — the compute-only mode, for benchmarking pure execution cost;
//   - a real committer (full or payloadless) drives a trie-backed ledger and produces proofs exactly
//     as the execution node does — for benchmarking the commit/proof-collection path.
//
// An in-memory execution-data provider is always used, so the execution-data root is still computed
// by production code but nothing durable is written.
//
// The manager reuses the production execution stack (`fvm.NewVirtualMachine`, the same FVM options as
// the execution node and `verify_execution_result`, and `computation.New`), so future changes to
// execution semantics are inherited automatically. It never wires a persistence step: computing a
// block returns a [github.com/onflow/flow-go/engine/execution.ComputationResult] in memory and writes
// nothing to any database.
//
// No error returns are expected during normal operation.
func NewComputationManager(
	logger zerolog.Logger,
	chainID flow.ChainID,
	headers storage.Headers,
	protoState protocol.State,
	viewCommitter computer.ViewCommitter,
	transactionFeesDisabled bool,
	scheduledTransactionsEnabled bool,
) (*computation.Manager, error) {
	me, err := throwawaySigner()
	if err != nil {
		return nil, fmt.Errorf("could not create throwaway signer: %w", err)
	}

	// FVM options: the same base options the execution node and verify_execution_result use. Note
	// that computation.New layers computation.DefaultFVMOptions on top of these internally, so we do
	// not add them here (that would double-apply them).
	fvmOptions := append(
		[]fvm.Option{fvm.WithLogger(logger)},
		initialize.InitFvmOptions(chainID, headers, transactionFeesDisabled)...,
	)
	fvmOptions = append(fvmOptions, fvm.WithScheduledTransactionsEnabled(scheduledTransactionsEnabled))
	vmCtx := fvm.NewContext(chainID.Chain(), fvmOptions...)

	// In-memory execution-data provider: the execution-data root is still computed by production code
	// (needed for a comparable ExecutionResult), but blobs are held in an in-memory datastore and no
	// tracker state is persisted.
	blobService := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	execDataProvider := provider.NewProvider(
		logger,
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		blobService,
		nil, // nil tracker storage -> tracker.NoopStorage: nothing durable is tracked
	)

	manager, err := computation.New(
		logger,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		me,
		computation.NewProtocolStateWrapper(protoState),
		vmCtx,
		viewCommitter,
		execDataProvider,
		computation.ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not create computation manager: %w", err)
	}
	return manager, nil
}

// throwawaySigner returns a [module.Local] backed by a deterministic throwaway staking key. The
// receipt-generation and SPOCK path in the block computer runs unchanged; the produced signatures do
// not match any real node identity, but state commitments and execution results — what benchmarks and
// comparisons care about — do not depend on the signer's identity.
//
// No error returns are expected during normal operation.
func throwawaySigner() (module.Local, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	if err != nil {
		return nil, fmt.Errorf("could not generate throwaway staking key: %w", err)
	}

	// A non-zero node ID is required: the receipt/SPOCK path rejects a zero executor ID. The value is
	// arbitrary because the produced signatures are not expected to match any real node identity.
	id := flow.IdentitySkeleton{
		NodeID:        flow.HashToID([]byte("reexecute-block-throwaway-executor")),
		Address:       "",
		Role:          flow.RoleExecution,
		InitialWeight: 1000,
		StakingPubKey: sk.PublicKey(),
	}

	me, err := local.New(id, sk)
	if err != nil {
		return nil, fmt.Errorf("could not create local signer: %w", err)
	}
	return me, nil
}
