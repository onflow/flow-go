package badger

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	statepkg "github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/datastore"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// cachedLatest caches both latest finalized and sealed block
// since finalized block and sealed block are updated together atomically,
// we can cache them together
type cachedLatest struct {
	finalizedID     flow.Identifier
	finalizedHeader *flow.Header
	sealedID        flow.Identifier
	sealedHeader    *flow.Header
}

type State struct {
	metrics     module.ComplianceMetrics
	db          storage.DB
	lockManager lockctx.Manager
	headers     storage.Headers
	blocks      storage.Blocks
	qcs         storage.QuorumCertificates
	results     storage.ExecutionResults
	seals       storage.Seals
	epoch       struct {
		setups  storage.EpochSetups
		commits storage.EpochCommits
	}
	params                      protocol.Params
	protocolKVStoreSnapshotsDB  storage.ProtocolKVStore
	epochProtocolStateEntriesDB storage.EpochProtocolStateEntries // TODO remove when MinEpochStateEntry is stored in KVStore
	protocolState               protocol.ProtocolState
	versionBeacons              storage.VersionBeacons

	// finalizedRootHeight marks the cutoff of the history this node knows about. We cache it in the state
	// because it cannot change over the lifecycle of a protocol state instance. It is frequently
	// larger than the height of the root block of the spork, (also cached below as
	// `sporkRootBlockHeight`), for instance, if the node joined in an epoch after the last spork.
	finalizedRootHeight uint64
	// sealedRootHeight returns the root block that is sealed. We cache it in
	// the state, because it cannot change over the lifecycle of a protocol state instance.
	sealedRootHeight uint64
	// sporkRootBlock is the root block in the current spork. We cache it in
	// the state, because it cannot change over the lifecycle of a protocol state instance.
	// Caution: A node that joined in a later epoch past the spork, the node will likely _not_
	// know the spork's root block in full (though it will always know the height).
	sporkRootBlock *flow.Block
	// cachedLatest caches both the *latest* finalized header and sealed header,
	// because the protocol state is solely responsible for updating it.
	// finalized header and sealed header can be cached together since they are updated together atomically
	cachedLatest *atomic.Pointer[cachedLatest]
}

var _ protocol.State = (*State)(nil)

type BootstrapConfig struct {
	// SkipNetworkAddressValidation flags allows skipping all the network address related
	// validations not needed for an unstaked node
	SkipNetworkAddressValidation bool
}

func defaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		SkipNetworkAddressValidation: false,
	}
}

type BootstrapConfigOptions func(conf *BootstrapConfig)

func SkipNetworkAddressValidation(conf *BootstrapConfig) {
	conf.SkipNetworkAddressValidation = true
}

func Bootstrap(
	metrics module.ComplianceMetrics,
	db storage.DB,
	lockManager lockctx.Manager,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	epochProtocolStateSnapshots storage.EpochProtocolStateEntries,
	protocolKVStoreSnapshots storage.ProtocolKVStore,
	versionBeacons storage.VersionBeacons,
	root protocol.Snapshot,
	options ...BootstrapConfigOptions,
) (*State, error) {
	// we acquire both [storage.LockInsertBlock] and [storage.LockFinalizeBlock] because
	// the bootstrapping process inserts and finalizes blocks (all blocks within the
	// trusted root snapshot are presumed to be finalized)
	lctx := lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertBlock)
	if err != nil {
		return nil, err
	}
	err = lctx.AcquireLock(storage.LockFinalizeBlock)
	if err != nil {
		return nil, err
	}

	config := defaultBootstrapConfig()
	for _, opt := range options {
		opt(config)
	}

	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if isBootstrapped {
		return nil, fmt.Errorf("expected empty database")
	}

	if err := datastore.IsValidRootSnapshot(root, !config.SkipNetworkAddressValidation); err != nil {
		return nil, fmt.Errorf("cannot bootstrap invalid root snapshot: %w", err)
	}

	segment, err := root.SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("could not get sealing segment: %w", err)
	}

	_, rootSeal, err := root.SealedResult()
	if err != nil {
		return nil, fmt.Errorf("could not get sealed result for sealing segment: %w", err)
	}

	// sealing segment is in ascending height order, so the tail is the
	// oldest ancestor and head is the newest child in the segment
	// TAIL <- ... <- HEAD
	lastFinalized := segment.Finalized() // the highest block in sealing segment is the last finalized block
	lastSealed := segment.Sealed()       // the lowest block in sealing segment is the last sealed block
	sporkRootBlock := segment.SporkRootBlock

	// bootstrap the sealing segment
	// creating sealed root block with the rootResult
	// creating finalized root block with lastFinalized
	err = bootstrapSealingSegment(lctx, db, blocks, qcs, segment, lastFinalized, rootSeal)
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap sealing chain segment blocks: %w", err)
	}

	err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// insert the root quorum certificate into the database
		qc, err := root.QuorumCertificate()
		if err != nil {
			return fmt.Errorf("could not get root qc: %w", err)
		}
		err = qcs.BatchStore(lctx, rw, qc)
		if err != nil {
			return fmt.Errorf("could not insert root qc: %w", err)
		}

		// initialize spork params
		err = bootstrapSporkInfo(lctx, rw, blocks, sporkRootBlock)
		if err != nil {
			return fmt.Errorf("could not bootstrap spork info: %w", err)
		}

		// bootstrap dynamic protocol state
		err = bootstrapProtocolState(lctx, rw, segment, root.Params(), epochProtocolStateSnapshots, protocolKVStoreSnapshots, setups, commits, !config.SkipNetworkAddressValidation)
		if err != nil {
			return fmt.Errorf("could not bootstrap protocol state: %w", err)
		}

		// initialize version beacon
		err = boostrapVersionBeacon(rw, root)
		if err != nil {
			return fmt.Errorf("could not bootstrap version beacon: %w", err)
		}

		err = updateEpochMetrics(metrics, root)
		if err != nil {
			return fmt.Errorf("could not update epoch metrics: %w", err)
		}
		metrics.BlockSealed(lastSealed)
		metrics.SealedHeight(lastSealed.Height)
		metrics.FinalizedHeight(lastFinalized.Height)
		for _, proposal := range segment.Blocks {
			metrics.BlockFinalized(&proposal.Block)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed: %w", err)
	}

	// The reason bootstrapStatePointers is the last step is that it
	// will Insert Finalized Height, which is used to determine if
	// the database has been bootstrapped.
	err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// initialize the current protocol state height/view pointers
		return bootstrapStatePointers(lctx, rw, root)
	})
	if err != nil {
		return nil, fmt.Errorf("could not bootstrap height/view pointers: %w", err)
	}

	instanceParams, err := datastore.ReadInstanceParams(db.Reader(), headers, seals)
	if err != nil {
		return nil, fmt.Errorf("could not read instance params: %w", err)
	}

	params := &datastore.Params{
		GlobalParams:   root.Params(),
		InstanceParams: instanceParams,
	}

	return newState(
		metrics,
		db,
		lockManager,
		headers,
		seals,
		results,
		blocks,
		qcs,
		setups,
		commits,
		epochProtocolStateSnapshots,
		protocolKVStoreSnapshots,
		versionBeacons,
		params,
		sporkRootBlock,
	)
}

// bootstrapProtocolStates bootstraps data structures needed for Dynamic Protocol State.
// The sealing segment may contain blocks committing to different Protocol State entries,
// in which case each of these protocol state entries are stored in the database during
// bootstrapping.
// For each distinct protocol state entry, we also store the associated EpochSetup and
// EpochCommit service events.
func bootstrapProtocolState(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	segment *flow.SealingSegment,
	params protocol.GlobalParams,
	epochProtocolStateSnapshots storage.EpochProtocolStateEntries,
	protocolKVStoreSnapshots storage.ProtocolKVStore,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	verifyNetworkAddress bool,
) error {
	// The sealing segment contains a protocol state entry for every block in the segment, including the root block.
	for protocolStateID, stateEntry := range segment.ProtocolStateEntries {
		// Store the protocol KV Store entry
		err := protocolKVStoreSnapshots.BatchStore(lctx, rw, protocolStateID, &stateEntry.KVStore)
		if err != nil {
			return fmt.Errorf("could not store protocol state kvstore: %w", err)
		}

		// Store the epoch portion of the protocol state, including underlying EpochSetup/EpochCommit service events
		dynamicEpochProtocolState, err := inmem.NewEpochProtocolStateAdapter(
			inmem.UntrustedEpochProtocolStateAdapter{
				RichEpochStateEntry: stateEntry.EpochEntry,
				Params:              params,
			},
		)
		if err != nil {
			return fmt.Errorf("could not construct epoch protocol state adapter: %w", err)
		}
		err = bootstrapEpochForProtocolStateEntry(rw, epochProtocolStateSnapshots, epochSetups, epochCommits, dynamicEpochProtocolState, verifyNetworkAddress)
		if err != nil {
			return fmt.Errorf("could not store epoch service events for state entry (id=%x): %w", stateEntry.EpochEntry.ID(), err)
		}
	}

	for _, proposal := range segment.AllBlocks() {
		blockID := proposal.Block.ID()
		protocolStateEntryWrapper := segment.ProtocolStateEntries[proposal.Block.Payload.ProtocolStateID]
		err := epochProtocolStateSnapshots.BatchIndex(rw, blockID, protocolStateEntryWrapper.EpochEntry.ID())
		if err != nil {
			return fmt.Errorf("could not index root protocol state: %w", err)
		}
		err = protocolKVStoreSnapshots.BatchIndex(lctx, rw, blockID, proposal.Block.Payload.ProtocolStateID)
		if err != nil {
			return fmt.Errorf("could not index root kv store: %w", err)
		}
	}

	return nil
}

// bootstrapSealingSegment inserts all blocks and associated metadata for the protocol state root
// snapshot to disk. We proceed as follows:
//  1. we persist the auxiliary execution results from the sealing segment
//  2. persist extra blocks from the sealing segment; these blocks are below the history cut-off and
//     therefore not fully indexed (we only index the blocks by height).
//  3. persist sealing segment Blocks and properly populate all indices as if those blocks:
//     - blocks are index by their heights
//     - latest seale is indexed for each block
//     - children of each block is initialized with the set containing the child block
//  4. For the highest seal (`rootSeal`), we index the sealed result ID in the database.
//     This is necessary for the execution node to confirm that it is starting to execute from the
//     correct state.
func bootstrapSealingSegment(
	lctx lockctx.Proof,
	db storage.DB,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	segment *flow.SealingSegment,
	head *flow.Block,
	rootSeal *flow.Seal,
) error {
	// STEP 1: persist AUXILIARY EXECUTION RESULTS (should include the result sealed by segment.FirstSeal if that is not nil)
	err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		w := rw.Writer()
		for _, result := range segment.ExecutionResults {
			err := operation.InsertExecutionResult(w, result)
			if err != nil {
				return fmt.Errorf("could not insert execution result: %w", err)
			}
			err = operation.IndexExecutionResult(w, result.BlockID, result.ID())
			if err != nil {
				return fmt.Errorf("could not index execution result: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// STEP 2: persist EXTRA BLOCKS to the database
	// These blocks are _ancestors_ of `segment.Blocks`, i.e. below the history cut-off. Therefore, we only persist the extra blocks
	// and index them by height, while all the other indices are omitted, as they would potentially reference non-existent data.
	//
	// We PERSIST these blocks ONE-BY-ONE in order of increasing height,
	// emulating the process during normal operations, the the following reason:
	// * Execution Receipts are incorporated into blocks for bookkeeping when and which execution results the ENs published.
	// * Typically, most ENs commit to the same results. Therefore, Results in blocks are stored separately from the Receipts
	//   in blocks and deduplicated along the fork -- specifically, we only store the result along a fork in the first block
	//   containing an execution receipt committing to that result. For receipts committing to the same result in descending
	//   blocks, we only store the receipt and omit the result as it is already contained in an ancestor.
	// * We want to ensure that for every receipt in a block that we store, the result is also going to be available in storage
	//   [Blocks.BatchStore] automatically performs this check and errors when attempting to store a block referencing unknown
	//   results.
	// * During normal operations, we ingest and persist blocks one by one. However, during bootstrapping we need to store
	//   multiple blocks. Hypothetically, if we were to store all blocks in the same batch, results included in ancestor blocks
	//   would not be persisted in the database yet when attempting to persist their descendants. In other words, the check in
	//   [Blocks.BatchStore] can't distinguish between a receipt referencing a missing result vs a receipt referencing a result
	//   that is contained in a previous block being stored as part of the same batch.
	// why not storing all blocks in the same batch?
	// because we need to ensure a block does not include a result that refers a unknown block.
	// when validating the results, we could check if the referred block exists in the database,
	// however if we are storing multiple blocks in the same batch, the previous block from the same
	// batch has not been stored in database yet, so the check can't distinguish between a block that
	// refers to a previous block in the same batch and a block that refers to a block that does not
	// exist in the database. Unless we pass down the previous blocks to the validation function as well
	// as the database operation functions, such as blocks.BatchStore method, which we consider a bad
	// practice, since having the database operation function taking previous blocks (or previous results)
	// as an argument is confusing and vulnerable to bugs.
	// since storing multiple blocks in the same batch only happens during bootstrapping, we decided to
	// store each block in a separate batch.
	for _, proposal := range segment.ExtraBlocks {
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			blockID := proposal.Block.ID()
			height := proposal.Block.Height
			err := blocks.BatchStore(lctx, rw, proposal)
			if err != nil {
				return fmt.Errorf("could not insert SealingSegment extra block: %w", err)
			}
			err = operation.IndexFinalizedBlockByHeight(lctx, rw, height, blockID)
			if err != nil {
				return fmt.Errorf("could not index SealingSegment extra block (id=%x): %w", blockID, err)
			}

			if proposal.Block.ContainsParentQC() {
				err = qcs.BatchStore(lctx, rw, proposal.Block.ParentQC())
				if err != nil {
					return fmt.Errorf("could not store qc for SealingSegment extra block (id=%x): %w", blockID, err)
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	// STEP 3: persist sealing segment Blocks and properly populate all indices as if those blocks
	// For each block B, we index the highest seal in the fork with head B. To sanity check proper state construction, we want to ensure that the referenced
	// seal actually exists in the database at the end of the bootstrapping process. Therefore, we track all the seals that we are storing and error in case
	// we attempt to reference a seal that is not in that set. It is fine to omit any seals in `segment.ExtraBlocks` for the following reason:
	//  * Let's consider the lowest-height block in `segment.Blocks`, by convention `segment.Blocks[0]`, and call it B1.
	//  * If B1 contains seals, then the latest seal as of B1 is part of the block's payload. S1 will be stored in the database while persisting B1.
	//  * If and only if B1 contains no seal, then `segment.FirstSeal` is set to the latest seal included in an ancestor of B1 (see [flow.SealingSegment]
	//    documentation). We explicitly store FirstSeal in the database.
	//  * By induction, this argument can be applied to all subsequent blocks in `segment.Blocks`. Hence, the index `LatestSealAtBlock` is correctly populated
	//    for all blocks in `segment.Blocks`.
	sealsLookup := make(map[flow.Identifier]struct{})
	sealsLookup[rootSeal.ID()] = struct{}{}
	if segment.FirstSeal != nil { // in case the segment's first block contains no seal, insert the first seal
		sealsLookup[segment.FirstSeal.ID()] = struct{}{}
		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			if segment.FirstSeal != nil {
				err := operation.InsertSeal(rw.Writer(), segment.FirstSeal.ID(), segment.FirstSeal)
				if err != nil {
					return fmt.Errorf("could not insert first seal: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// PERSIST these blocks ONE-BY-ONE in order of increasing height, emulating the process during normal operations,
	// so sanity checks from normal operations should continue to apply.
	for i, proposal := range segment.Blocks {
		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			w := rw.Writer()
			blockID := proposal.Block.ID()
			height := proposal.Block.Height

			err := blocks.BatchStore(lctx, rw, proposal)
			if err != nil {
				return fmt.Errorf("could not insert SealingSegment block: %w", err)
			}
			err = operation.IndexFinalizedBlockByHeight(lctx, rw, height, blockID)
			if err != nil {
				return fmt.Errorf("could not index SealingSegment block (id=%x): %w", blockID, err)
			}

			if proposal.Block.ContainsParentQC() {
				err = qcs.BatchStore(lctx, rw, proposal.Block.ParentQC())
				if err != nil {
					return fmt.Errorf("could not store qc for SealingSegment block (id=%x): %w", blockID, err)
				}
			}

			// index the latest seal as of this block
			latestSealID, ok := segment.LatestSeals[blockID]
			if !ok {
				return fmt.Errorf("missing latest seal for sealing segment block (id=%s)", blockID)
			}

			// build seals lookup
			for _, seal := range proposal.Block.Payload.Seals {
				sealsLookup[seal.ID()] = struct{}{}
			}
			// sanity check: make sure the seal exists
			_, ok = sealsLookup[latestSealID]
			if !ok {
				return fmt.Errorf("sanity check fail: missing latest seal for sealing segment block (id=%s)", blockID)
			}
			err = operation.IndexLatestSealAtBlock(lctx, w, blockID, latestSealID)
			if err != nil {
				return fmt.Errorf("could not index block seal: %w", err)
			}

			// for all but the first block in the segment, index the parent->child relationship
			// Populate parent->child relationship
			if i > 0 {
				err = operation.UpsertBlockChildren(lctx, w, proposal.Block.ParentID, []flow.Identifier{blockID})
				if err != nil {
					return fmt.Errorf("could not insert child index for block (id=%x): %w", blockID, err)
				}
			}
			if i == len(segment.Blocks)-1 { // in addition, for the highest block in the sealing segment, the known set of children is empty:
				err = operation.UpsertBlockChildren(lctx, rw.Writer(), head.ID(), nil)
				if err != nil {
					return fmt.Errorf("could not insert child index for head block (id=%x): %w", head.ID(), err)
				}
			}

			return nil
		})
		if err != nil {
			return err
		}
	}

	// STEP 4: For the highest seal (`rootSeal`), we index the sealed result ID in the database.
	err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// sanity check existence of referenced execution result (should have been stored in STEP 1)
		var result flow.ExecutionResult
		err := operation.RetrieveExecutionResult(rw.GlobalReader(), rootSeal.ResultID, &result)
		if err != nil {
			return fmt.Errorf("missing sealed execution result %v: %w", rootSeal.ResultID, err)
		}

		// If the sealed root block is different from the finalized root block, then it means the node dynamically
		// bootstrapped. In that case, we index the result of the latest sealed result, so that the EN is able
		// to confirm that it is loading the correct state to execute the next block.
		err = operation.IndexExecutionResult(rw.Writer(), rootSeal.BlockID, rootSeal.ResultID)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// bootstrapStatePointers instantiates central pointers used to by the protocol
// state for keeping track of lifecycle variables:
//   - Consensus Safety and Liveness Data (only used by consensus participants)
//   - Root Block's Height (heighest block in sealing segment)
//   - Sealed Root Block Height (block height sealed as of the Root Block)
//   - Latest Finalized Height (initialized to height of Root Block)
//   - Latest Sealed Block Height (initialized to block height sealed as of the Root Block)
//   - initial entry in map:
//     Finalized Block ID -> ID of latest seal in fork with this block as head
func bootstrapStatePointers(lctx lockctx.Proof, rw storage.ReaderBatchWriter, root protocol.Snapshot) error {
	// sealing segment lists blocks in order of ascending height, so the tail
	// is the oldest ancestor and head is the newest child in the segment
	// TAIL <- ... <- HEAD
	segment, err := root.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not get sealing segment: %w", err)
	}
	highest := segment.Finalized() // the highest block in sealing segment is the last finalized block
	lowest := segment.Sealed()     // the lowest block in sealing segment is the last sealed block

	// find the finalized seal that seals the lowest block, meaning seal.BlockID == lowest.ID()
	seal, err := segment.FinalizedSeal()
	if err != nil {
		return fmt.Errorf("could not get finalized seal from sealing segment: %w", err)
	}

	safetyData := &hotstuff.SafetyData{
		LockedOneChainView:      highest.View,
		HighestAcknowledgedView: highest.View,
	}

	// Per convention, all blocks in the sealing segment must be finalized. Therefore, a QC must
	// exist for the `highest` block in the sealing segment. The QC for `highest` should be
	// contained in the `root` Snapshot and returned by `root.QuorumCertificate()`. Otherwise,
	// the Snapshot is incomplete, because consensus nodes require this QC. To reduce the chance of
	// accidental misconfiguration undermining consensus liveness, we do the following sanity checks:
	//  * `rootQC` should not be nil
	//  * `rootQC` should be for `highest` block, i.e. its view and blockID should match
	rootQC, err := root.QuorumCertificate()
	if err != nil {
		return fmt.Errorf("could not get root QC: %w", err)
	}
	if rootQC == nil {
		return fmt.Errorf("QC for highest (finalized) block in sealing segment cannot be nil")
	}
	if rootQC.View != highest.View {
		return fmt.Errorf("root QC's view %d does not match the highest block in sealing segment (view %d)", rootQC.View, highest.View)
	}
	if rootQC.BlockID != highest.ID() {
		return fmt.Errorf("root QC is for block %v, which does not match the highest block %v in sealing segment", rootQC.BlockID, highest.ID())
	}

	livenessData := &hotstuff.LivenessData{
		CurrentView: highest.View + 1,
		NewestQC:    rootQC,
	}

	w := rw.Writer()
	// insert initial views for HotStuff
	err = operation.UpsertSafetyData(w, highest.ChainID, safetyData)
	if err != nil {
		return fmt.Errorf("could not insert safety data: %w", err)
	}
	err = operation.UpsertLivenessData(w, highest.ChainID, livenessData)
	if err != nil {
		return fmt.Errorf("could not insert liveness data: %w", err)
	}

	// insert height pointers
	err = operation.InsertRootHeight(w, highest.Height)
	if err != nil {
		return fmt.Errorf("could not insert finalized root height: %w", err)
	}
	// the sealed root height is the lowest block in sealing segment
	err = operation.InsertSealedRootHeight(w, lowest.Height)
	if err != nil {
		return fmt.Errorf("could not insert sealed root height: %w", err)
	}
	err = operation.UpsertFinalizedHeight(lctx, w, highest.Height)
	if err != nil {
		return fmt.Errorf("could not insert finalized height: %w", err)
	}
	err = operation.UpsertSealedHeight(lctx, w, lowest.Height)
	if err != nil {
		return fmt.Errorf("could not insert sealed height: %w", err)
	}
	err = operation.IndexFinalizedSealByBlockID(w, seal.BlockID, seal.ID())
	if err != nil {
		return fmt.Errorf("could not index sealed block: %w", err)
	}

	// insert first-height indices for epochs which begin within the sealing segment
	err = indexEpochHeights(lctx, rw, segment)
	if err != nil {
		return fmt.Errorf("could not index epoch heights: %w", err)
	}

	return nil
}

// bootstrapEpochForProtocolStateEntry bootstraps the protocol state database with epoch
// information (in particular, EpochSetup and EpochCommit service events) associated with
// a particular Dynamic Protocol State entry.
// There may be several such entries within a single root snapshot, in which case this
// function is called once for each entry. Entries may overlap in which underlying
// epoch information (service events) they reference -- this only has a minor performance
// cost, as duplicate writes of the same data are idempotent.
func bootstrapEpochForProtocolStateEntry(
	rw storage.ReaderBatchWriter,
	epochProtocolStateSnapshots storage.EpochProtocolStateEntries,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	epochProtocolStateEntry protocol.EpochProtocolState,
	verifyNetworkAddress bool,
) error {
	richEntry := epochProtocolStateEntry.Entry()

	// keep track of EpochSetup/EpochCommit service events, then store them after this step is complete
	var setups []*flow.EpochSetup
	var commits []*flow.EpochCommit

	// validate and insert previous epoch if it exists
	if epochProtocolStateEntry.PreviousEpochExists() {
		// if there is a previous epoch, both setup and commit events must exist
		setup := richEntry.PreviousEpochSetup
		commit := richEntry.PreviousEpochCommit

		if err := protocol.IsValidEpochSetup(setup, verifyNetworkAddress); err != nil {
			return fmt.Errorf("invalid EpochSetup for previous epoch: %w", err)
		}
		if err := protocol.IsValidEpochCommit(commit, setup); err != nil {
			return fmt.Errorf("invalid EpochCommit for previous epoch: %w", err)
		}

		setups = append(setups, setup)
		commits = append(commits, commit)
	}

	{ // validate and insert current epoch (always exist)
		setup := richEntry.CurrentEpochSetup
		commit := richEntry.CurrentEpochCommit

		if err := protocol.IsValidEpochSetup(setup, verifyNetworkAddress); err != nil {
			return fmt.Errorf("invalid EpochSetup for current epoch: %w", err)
		}
		if err := protocol.IsValidEpochCommit(commit, setup); err != nil {
			return fmt.Errorf("invalid EpochCommit for current epoch: %w", err)
		}

		setups = append(setups, setup)
		commits = append(commits, commit)
	}

	// validate and insert next epoch, if it exists
	if richEntry.NextEpoch != nil {
		setup := richEntry.NextEpochSetup   // must not be nil
		commit := richEntry.NextEpochCommit // may be nil

		if err := protocol.IsValidEpochSetup(setup, verifyNetworkAddress); err != nil {
			return fmt.Errorf("invalid EpochSetup for next epoch: %w", err)
		}
		setups = append(setups, setup)

		if commit != nil {
			if err := protocol.IsValidEpochCommit(commit, setup); err != nil {
				return fmt.Errorf("invalid EpochCommit for next epoch: %w", err)
			}
			commits = append(commits, commit)
		}
	}

	// insert all epoch setup/commit service events
	// dynamic protocol state relies on these events being stored
	for _, setup := range setups {
		err := epochSetups.BatchStore(rw, setup)
		if err != nil {
			return fmt.Errorf("could not store epoch setup event: %w", err)
		}
	}
	for _, commit := range commits {
		err := epochCommits.BatchStore(rw, commit)
		if err != nil {
			return fmt.Errorf("could not store epoch commit event: %w", err)
		}
	}

	// insert epoch protocol state entry, which references above service events
	err := epochProtocolStateSnapshots.BatchStore(rw.Writer(), richEntry.ID(), richEntry.MinEpochStateEntry)
	if err != nil {
		return fmt.Errorf("could not store epoch protocol state entry: %w", err)
	}
	return nil
}

// bootstrapSporkInfo bootstraps the protocol state with information about the
// spork which is used to disambiguate Flow networks.
func bootstrapSporkInfo(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blocks storage.Blocks,
	sporkRootBlock *flow.Block,
) error {

	w := rw.Writer()
	sporkRootBlockID := sporkRootBlock.ID()
	// store the spork root block ID
	err := operation.IndexSporkRootBlock(w, sporkRootBlockID)
	if err != nil {
		return fmt.Errorf("could not insert spork ID: %w", err)
	}

	proposal, err := flow.NewRootProposal(
		flow.UntrustedProposal{
			Block:           *sporkRootBlock,
			ProposerSigData: nil,
		},
	)
	if err != nil {
		return fmt.Errorf("could not create root proposal for spork root block: %w", err)
	}

	err = blocks.BatchStore(lctx, rw, proposal)
	if err != nil {
		return fmt.Errorf("could not store spork root block: %w", err)
	}

	return nil
}

// indexEpochHeights populates the epoch height index from the root snapshot.
// We index the FirstHeight for every epoch where the transition occurs within the sealing segment of the root snapshot,
// or for the first epoch of a spork if the snapshot is a spork root snapshot (1 block sealing segment).
// No errors are expected during normal operation.
func indexEpochHeights(lctx lockctx.Proof, rw storage.ReaderBatchWriter, segment *flow.SealingSegment) error {
	// CASE 1: For spork root snapshots, there is exactly one block B and one epoch E.
	// Index `E.counter → B.Height`.
	if segment.IsSporkRoot() {
		counter := segment.LatestProtocolStateEntry().EpochEntry.EpochCounter()
		firstHeight := segment.Highest().Height
		err := operation.InsertEpochFirstHeight(lctx, rw, counter, firstHeight)
		if err != nil {
			return fmt.Errorf("could not index first height %d for epoch %d: %w", firstHeight, counter, err)
		}
		return nil
	}

	// CASE 2: For all other snapshots, there is a segment of blocks which may span several epochs.
	// We traverse all blocks in the segment in ascending height order.
	// If we find two consecutive blocks B1, B2 so that `B1.EpochCounter` != `B2.EpochCounter`,
	// then index `B2.EpochCounter → B2.Height`.
	allBlocks := segment.AllBlocks()
	lastBlock := allBlocks[0]
	lastBlockEpochCounter := segment.ProtocolStateEntries[lastBlock.Block.Payload.ProtocolStateID].EpochEntry.EpochCounter()
	for _, block := range allBlocks[1:] {
		thisBlockEpochCounter := segment.ProtocolStateEntries[block.Block.Payload.ProtocolStateID].EpochEntry.EpochCounter()
		if lastBlockEpochCounter != thisBlockEpochCounter {
			firstHeight := block.Block.Height
			err := operation.InsertEpochFirstHeight(lctx, rw, thisBlockEpochCounter, firstHeight)
			if err != nil {
				return fmt.Errorf("could not index first height %d for epoch %d: %w", firstHeight, thisBlockEpochCounter, err)
			}
		}
		lastBlockEpochCounter = thisBlockEpochCounter
	}
	return nil
}

func OpenState(
	metrics module.ComplianceMetrics,
	db storage.DB,
	lockManager lockctx.Manager,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	epochProtocolState storage.EpochProtocolStateEntries,
	protocolKVStoreSnapshots storage.ProtocolKVStore,
	versionBeacons storage.VersionBeacons,
) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	sporkRootBlock, err := ReadSporkRootBlock(db, blocks)
	if err != nil {
		return nil, fmt.Errorf("could not read spork root block: %w", err)
	}
	globalParams := inmem.NewParams(
		inmem.EncodableParams{
			ChainID:              sporkRootBlock.ChainID,
			SporkID:              sporkRootBlock.ID(),
			SporkRootBlockHeight: sporkRootBlock.Height,
			SporkRootBlockView:   sporkRootBlock.View,
		},
	)
	instanceParams, err := ReadInstanceParams(db, headers, seals)
	if err != nil {
		return nil, fmt.Errorf("could not read instance params: %w", err)
	}
	params := &datastore.Params{
		GlobalParams:   globalParams,
		InstanceParams: instanceParams,
	}

	state, err := newState(
		metrics,
		db,
		lockManager,
		headers,
		seals,
		results,
		blocks,
		qcs,
		setups,
		commits,
		epochProtocolState,
		protocolKVStoreSnapshots,
		versionBeacons,
		params,
		sporkRootBlock,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create state: %w", err)
	}

	// report last finalized and sealed block height
	finalSnapshot := state.Final()
	head, err := finalSnapshot.Head()
	if err != nil {
		return nil, fmt.Errorf("unexpected error to get finalized block: %w", err)
	}
	metrics.FinalizedHeight(head.Height)

	sealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get latest sealed block: %w", err)
	}
	metrics.SealedHeight(sealed.Height)

	err = updateEpochMetrics(metrics, finalSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to update epoch metrics: %w", err)
	}

	return state, nil
}

func (state *State) Params() protocol.Params {
	return state.params
}

// Sealed returns a snapshot for the latest sealed block. A latest sealed block
// must always exist, so this function always returns a valid snapshot.
func (state *State) Sealed() protocol.Snapshot {
	cached := state.cachedLatest.Load()
	if cached == nil {
		return invalid.NewSnapshotf("internal inconsistency: no cached sealed header")
	}
	return NewFinalizedSnapshot(state, cached.sealedID, cached.sealedHeader)
}

// Final returns a snapshot for the latest finalized block. A latest finalized
// block must always exist, so this function always returns a valid snapshot.
func (state *State) Final() protocol.Snapshot {
	cached := state.cachedLatest.Load()
	if cached == nil {
		return invalid.NewSnapshotf("internal inconsistency: no cached final header")
	}
	return NewFinalizedSnapshot(state, cached.finalizedID, cached.finalizedHeader)
}

// AtHeight returns a snapshot for the finalized block at the given height.
// This function may return an invalid.Snapshot with:
//   - state.ErrUnknownSnapshotReference:
//     -> if no block with the given height has been finalized, even if it is incorporated
//     -> if the given height is below the root height
//   - exception for critical unexpected storage errors
func (state *State) AtHeight(height uint64) protocol.Snapshot {
	blockID, err := state.headers.BlockIDByHeight(height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return invalid.NewSnapshotf("unknown finalized height %d: %w", height, statepkg.ErrUnknownSnapshotReference)
		}
		// critical storage error
		return invalid.NewSnapshotf("could not look up block by height: %w", err)
	}
	return newSnapshotWithIncorporatedReferenceBlock(state, blockID)
}

// AtBlockID returns a snapshot for the block with the given ID. The block may be
// finalized or un-finalized.
// This function may return an invalid.Snapshot with:
//   - state.ErrUnknownSnapshotReference:
//     -> if no block with the given ID exists in the state
//   - exception for critical unexpected storage errors
func (state *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	exists, err := state.headers.Exists(blockID)
	if err != nil {
		return invalid.NewSnapshotf("could not check existence of reference block: %w", err)
	}
	if !exists {
		return invalid.NewSnapshotf("unknown block %x: %w", blockID, statepkg.ErrUnknownSnapshotReference)
	}
	return newSnapshotWithIncorporatedReferenceBlock(state, blockID)
}

// newState initializes a new state backed by the provided a badger database,
// mempools and service components.
// The parameter `expectedBootstrappedState` indicates whether the database
// is expected to contain an already bootstrapped state or not
func newState(
	metrics module.ComplianceMetrics,
	db storage.DB,
	lockManager lockctx.Manager,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	epochProtocolStateSnapshots storage.EpochProtocolStateEntries,
	protocolKVStoreSnapshots storage.ProtocolKVStore,
	versionBeacons storage.VersionBeacons,
	params protocol.Params,
	sporkRootBlock *flow.Block,
) (*State, error) {
	state := &State{
		metrics:     metrics,
		db:          db,
		lockManager: lockManager,
		headers:     headers,
		results:     results,
		seals:       seals,
		blocks:      blocks,
		qcs:         qcs,
		epoch: struct {
			setups  storage.EpochSetups
			commits storage.EpochCommits
		}{
			setups:  setups,
			commits: commits,
		},
		params:                      params,
		protocolKVStoreSnapshotsDB:  protocolKVStoreSnapshots,
		epochProtocolStateEntriesDB: epochProtocolStateSnapshots,
		protocolState: protocol_state.
			NewProtocolState(
				epochProtocolStateSnapshots,
				protocolKVStoreSnapshots,
				params,
			),
		versionBeacons: versionBeacons,
		cachedLatest:   new(atomic.Pointer[cachedLatest]),
		sporkRootBlock: sporkRootBlock,
	}

	// populate the protocol state cache
	err := state.populateCache()
	if err != nil {
		return nil, fmt.Errorf("failed to populate cache: %w", err)
	}

	return state, nil
}

// IsBootstrapped returns whether the database contains a bootstrapped state
func IsBootstrapped(db storage.DB) (bool, error) {
	var finalized uint64
	err := operation.RetrieveFinalizedHeight(db.Reader(), &finalized)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}

// updateEpochMetrics update the `consensus_compliance_current_epoch_counter` and the
// `consensus_compliance_current_epoch_phase` metric
func updateEpochMetrics(metrics module.ComplianceMetrics, snap protocol.Snapshot) error {
	currentEpoch, err := snap.Epochs().Current()
	if err != nil {
		return fmt.Errorf("could not get current epoch: %w", err)
	}
	metrics.CurrentEpochCounter(currentEpoch.Counter())
	metrics.CurrentEpochFinalView(currentEpoch.FinalView())
	metrics.CurrentDKGPhaseViews(currentEpoch.DKGPhase1FinalView(), currentEpoch.DKGPhase2FinalView(), currentEpoch.DKGPhase3FinalView())

	epochProtocolState, err := snap.EpochProtocolState()
	if err != nil {
		return fmt.Errorf("could not get epoch protocol state: %w", err)
	}
	metrics.CurrentEpochPhase(epochProtocolState.EpochPhase()) // update epoch phase
	// notify whether epoch fallback mode is active
	if epochProtocolState.EpochFallbackTriggered() {
		metrics.EpochFallbackModeTriggered()
	}

	return nil
}

// boostrapVersionBeacon bootstraps version beacon, by adding the latest beacon
// to an index, if present.
func boostrapVersionBeacon(rw storage.ReaderBatchWriter, snapshot protocol.Snapshot) error {
	versionBeacon, err := snapshot.VersionBeacon()
	if err != nil {
		return err
	}
	if versionBeacon == nil {
		return nil
	}
	return operation.IndexVersionBeaconByHeight(rw.Writer(), versionBeacon)
}

// populateCache is used after opening or bootstrapping the state to populate the cache.
// The cache must be populated before the State receives any queries.
// No errors expected during normal operations.
func (state *State) populateCache() error {
	// cache the initial value for finalized block
	// finalized header
	r := state.db.Reader()
	var finalizedHeight uint64
	err := operation.RetrieveFinalizedHeight(r, &finalizedHeight)
	if err != nil {
		return fmt.Errorf("could not lookup finalized height: %w", err)
	}
	var cachedLatest cachedLatest
	err = operation.LookupBlockHeight(r, finalizedHeight, &cachedLatest.finalizedID)
	if err != nil {
		return fmt.Errorf("could not lookup finalized id (height=%d): %w", finalizedHeight, err)
	}
	cachedLatest.finalizedHeader, err = state.headers.ByBlockID(cachedLatest.finalizedID)
	if err != nil {
		return fmt.Errorf("could not get finalized block (id=%x): %w", cachedLatest.finalizedID, err)
	}
	// sealed header
	var sealedHeight uint64
	err = operation.RetrieveSealedHeight(r, &sealedHeight)
	if err != nil {
		return fmt.Errorf("could not lookup sealed height: %w", err)
	}
	err = operation.LookupBlockHeight(r, sealedHeight, &cachedLatest.sealedID)
	if err != nil {
		return fmt.Errorf("could not lookup sealed id (height=%d): %w", sealedHeight, err)
	}
	cachedLatest.sealedHeader, err = state.headers.ByBlockID(cachedLatest.sealedID)
	if err != nil {
		return fmt.Errorf("could not get sealed block (id=%x): %w", cachedLatest.sealedID, err)
	}
	state.cachedLatest.Store(&cachedLatest)

	state.finalizedRootHeight = state.Params().FinalizedRoot().Height
	state.sealedRootHeight = state.Params().SealedRoot().Height

	return nil
}
