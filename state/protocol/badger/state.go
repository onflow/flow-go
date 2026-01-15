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

// Bootstrap initializes the protocol state from the provided root snapshot and persists it to the database.
// No errors expected during normal operation.
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

	// sealing segment lists blocks in order of ascending height, so the tail
	// is the oldest ancestor and head is the newest child in the segment
	// TAIL <- ... <- HEAD
	// Per definition, the highest block in the sealing segment is the last finalized block.
	// (The lowest block in sealing segment is the last sealed block, but we don't use that here.)

	// Overview of ACQUIRED LOCKS:
	//  * In a nutshell, the instance parameters describe how far back the node's local history reaches in comparison
	//    to the genesis / spork root block. These values are immutable throughout the lifetime of a node. Hence, the
	//    lock [storage.LockInsertInstanceParams] should only ever be used here during bootstrapping.
	//  * We acquire both [storage.LockInsertBlock] and [storage.LockFinalizeBlock] because the bootstrapping process
	//    inserts and finalizes blocks (all blocks within the trusted root snapshot are presumed to be finalized).
	//  * Liveness and safety data for Jolteon consensus need to be initialized, requiring [storage.LockInsertSafetyData]
	//    and [storage.LockInsertLivenessData].
	//  * The lock [storage.LockIndexExecutionResult] is required for bootstrapping execution. When bootstrapping other
	//    node roles, the lock is acquired (for algorithmic uniformity) but not used.
	err = storage.WithLocks(lockManager, storage.LockGroupProtocolStateBootstrap, func(lctx lockctx.Context) error {
		// bootstrap the sealing segment
		// creating sealed root block with the rootResult
		// creating finalized root block with lastFinalized
		err = bootstrapSealingSegment(lctx, db, blocks, qcs, segment, rootSeal)
		if err != nil {
			return fmt.Errorf("could not bootstrap sealing chain segment blocks: %w", err)
		}

		// bootstrap dynamic protocol state
		err = bootstrapProtocolState(lctx, db, segment, root.Params(), epochProtocolStateSnapshots, protocolKVStoreSnapshots, setups, commits, !config.SkipNetworkAddressValidation)
		if err != nil {
			return fmt.Errorf("could not bootstrap protocol state: %w", err)
		}

		// initialize version beacon
		err = boostrapVersionBeacon(db, root)
		if err != nil {
			return fmt.Errorf("could not bootstrap version beacon: %w", err)
		}

		// CAUTION: INSERT FINALIZED HEIGHT must be LAST, because we use its existence in the database
		// as indicator that the protocol database has been bootstrapped successfully. Before we write the
		// final piece of data to complete the bootstrapping, we query the current state of the database
		// (sanity check) to ensure that it is still considered as not properly bootstrapped.
		isBootstrapped, err = IsBootstrapped(db)
		if err != nil {
			return fmt.Errorf("determining whether database is successfully bootstrapped failed with unexpected exception: %w", err)
		}
		if isBootstrapped { // we haven't written the latest finalized height yet, so this value must be false
			return fmt.Errorf("sanity check failed: while bootstrapping has not yet completed, the implementation already considers the protocol state as successfully bootstrapped")
		}
		// initialize the current protocol state height/view pointers
		err = bootstrapStatePointers(lctx, db, root)
		if err != nil {
			return fmt.Errorf("could not bootstrap height/view pointers: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	state, err := OpenState(metrics, db, lockManager, headers, seals, results, blocks, qcs, setups, commits, epochProtocolStateSnapshots, protocolKVStoreSnapshots, versionBeacons)
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed, because the resulting database state is rejected: %w", err)
	}
	return state, nil
}

// bootstrapProtocolStates bootstraps data structures needed for Dynamic Protocol State.
// The sealing segment may contain blocks committing to different Protocol State entries,
// in which case each of these protocol state entries are stored in the database during
// bootstrapping. For each distinct protocol state entry, we also store the associated
// EpochSetup and EpochCommit service events.
//
// Caller must hold [storage.LockInsertBlock] lock.
//
// No error returns expected during normal operation.
func bootstrapProtocolState(
	lctx lockctx.Proof,
	db storage.DB,
	segment *flow.SealingSegment,
	params protocol.GlobalParams,
	epochProtocolStateSnapshots storage.EpochProtocolStateEntries,
	protocolKVStoreSnapshots storage.ProtocolKVStore,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	verifyNetworkAddress bool,
) error {
	return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// The sealing segment contains a protocol state entry for every block in the segment, including the root block.
		for protocolStateID, stateEntry := range segment.ProtocolStateEntries {
			// Store the protocol KV Store entry
			err := protocolKVStoreSnapshots.BatchStore(rw, protocolStateID, &stateEntry.KVStore)
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
			err := epochProtocolStateSnapshots.BatchIndex(lctx, rw, blockID, protocolStateEntryWrapper.EpochEntry.ID())
			if err != nil {
				return fmt.Errorf("could not index root protocol state: %w", err)
			}
			err = protocolKVStoreSnapshots.BatchIndex(lctx, rw, blockID, proposal.Block.Payload.ProtocolStateID)
			if err != nil {
				return fmt.Errorf("could not index root kv store: %w", err)
			}
		}

		return nil
	})
}

// bootstrapSealingSegment inserts all blocks and associated metadata for the protocol state root
// snapshot to disk. We proceed as follows:
//  1. we persist the auxiliary execution results from the sealing segment
//  2. persist extra blocks from the sealing segment; these blocks are below the history cut-off and
//     therefore not fully indexed (we only index the blocks by height).
//  3. persist sealing segment Blocks and properly populate all indices of those blocks:
//     - blocks are indexed by their heights
//     - latest seal is indexed for each block
//     - children of each block are initialized with the set containing the child block
//  4. For the highest seal (`rootSeal`), we index the sealed result ID in the database.
//     This is necessary for the execution node to confirm that it is starting to execute from the
//     correct state.
//  5. persist the spork root block. This block is always provided separately in the sealing
//     segment, as it may or may not be included in SealingSegment.Blocks depending on how much
//     history is covered. The spork root block is persisted as a root proposal without proposer
//     signature (by convention).
//
// Required locks: [storage.LockIndexExecutionResult] and [storage.LockInsertBlock] and [storage.LockFinalizeBlock]
func bootstrapSealingSegment(
	lctx lockctx.Proof,
	db storage.DB,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	segment *flow.SealingSegment,
	rootSeal *flow.Seal,
) error {
	// STEP 1: persist AUXILIARY EXECUTION RESULTS (should include the result sealed by segment.FirstSeal if that is not nil)
	err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		w := rw.Writer()
		for _, result := range segment.ExecutionResults {
			err := operation.InsertExecutionResult(w, result.ID(), result)
			if err != nil {
				return fmt.Errorf("could not insert execution result: %w", err)
			}
			err = operation.IndexTrustedExecutionResult(lctx, rw, result.BlockID, result.ID()) // requires [storage.LockIndexExecutionResult]
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
	// emulating the process during normal operations, the following reason:
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

			if proposal.Block.ContainsParentQC() { // Only spork root blocks or network genesis blocks do not contain a parent QC.
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

	// STEP 3: persist sealing segment Blocks and properly populate all indices as if those blocks were ingested during normal operations.
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

			// persist block and index it by height (all blocks in sealing segment are finalized by convention)
			err := blocks.BatchStore(lctx, rw, proposal)
			if err != nil {
				return fmt.Errorf("could not insert SealingSegment block: %w", err)
			}
			err = operation.IndexFinalizedBlockByHeight(lctx, rw, height, blockID)
			if err != nil {
				return fmt.Errorf("could not index SealingSegment block (id=%x): %w", blockID, err)
			}
			if proposal.Block.ContainsParentQC() { // Only spork root blocks or network genesis blocks do not contain a parent QC.
				err = qcs.BatchStore(lctx, rw, proposal.Block.ParentQC())
				if err != nil {
					return fmt.Errorf("could not store qc for SealingSegment block (id=%x): %w", blockID, err)
				}
			}

			// add seals in the block to our set of known seals (all of those will be persisted as part of storing the block)
			for _, seal := range proposal.Block.Payload.Seals {
				sealsLookup[seal.ID()] = struct{}{}
			}

			// index the latest seal as of this block
			latestSealID, ok := segment.LatestSeals[blockID]
			if !ok {
				return fmt.Errorf("missing latest seal for sealing segment block (id=%s)", blockID)
			}
			_, ok = sealsLookup[latestSealID] // sanity check: make sure that the latest seal as of this block is actually known
			if !ok {
				return fmt.Errorf("sanity check fail: missing latest seal for sealing segment block (id=%s)", blockID)
			}
			err = operation.IndexLatestSealAtBlock(lctx, w, blockID, latestSealID) // persist the mapping from block -> latest seal
			if err != nil {
				return fmt.Errorf("could not index block seal: %w", err)
			}

			// For all but the first block in the segment, index the parent->child relationship:
			if i > 0 {
				// Reason for skipping block at index i == 0:
				//  * `segment.Blocks[0]` is the node's root block, history prior to that root block is not guaranteed to be known to the node.
				//  * For consistency, we don't want to index children for an unknown or non-existent parent.
				//    So by convention, we start populating the parent-child relationship only for the root block's children and its descendants.
				//    This convention also covers the genesis block, where no parent exists.
				err = operation.IndexNewBlock(lctx, rw, blockID, proposal.Block.ParentID)
				if err != nil {
					return fmt.Errorf("could not index block (id=%x): %w", blockID, err)
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
		err = operation.IndexTrustedExecutionResult(lctx, rw, rootSeal.BlockID, rootSeal.ResultID)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// STEP 5: PERSIST spork root block
	// The spork root block is always provided by a sealing segment separately. This is because the spork root block
	// may or may not be part of [SealingSegment.Blocks] depending on how much history the sealing segment covers.
	sporkRootBlock := segment.SporkRootBlock

	// create the spork root proposal
	proposal, err := flow.NewRootProposal(
		flow.UntrustedProposal{
			Block:           *sporkRootBlock,
			ProposerSigData: nil, // by protocol convention, the spork root block (or genesis block) don't have a proposer signature
		},
	)
	if err != nil {
		return fmt.Errorf("could not create root proposal for spork root block: %w", err)
	}

	err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err = blocks.BatchStore(lctx, rw, proposal)
		if err != nil {
			// the spork root block may or may not have already been persisted, depending
			// on whether the root snapshot sealing segment contained it.
			if errors.Is(err, storage.ErrAlreadyExists) {
				return nil
			}
			return fmt.Errorf("could not store spork root block: %w", err)
		}

		return nil
	})

	return nil
}

// bootstrapStatePointers instantiates central pointers used to by the protocol
// state for keeping track of lifecycle variables:
//   - Consensus Safety and Liveness Data (only used by consensus participants)
//   - Root Block's Height (highest block in sealing segment)
//   - Sealed Root Block Height (block height sealed as of the Root Block)
//   - Latest Finalized Height (initialized to height of Root Block)
//   - Latest Sealed Block Height (initialized to block height sealed as of the Root Block)
//   - Spork root block ID (spork root block in sealing segment)
//   - initial entry in map:
//     Finalized Block ID -> ID of latest seal in fork with this block as head
//
// Caller must hold locks:
// [storage.LockInsertSafetyData] and [storage.LockInsertLivenessData] and
// [storage.LockInsertBlock] and [storage.LockFinalizeBlock]
//
// No error returns expected during normal operation.
func bootstrapStatePointers(lctx lockctx.Proof, db storage.DB, root protocol.Snapshot) error {
	// sealing segment lists blocks in order of ascending height, so the tail
	// is the oldest ancestor and head is the newest child in the segment
	// TAIL <- ... <- HEAD
	segment, err := root.SealingSegment()
	if err != nil {
		return fmt.Errorf("could not get sealing segment: %w", err)
	}
	lastFinalized := segment.Finalized() // the lastFinalized block in sealing segment is the latest known finalized block
	lastSealed := segment.Sealed()       // the lastSealed block in sealing segment is the latest known sealed block

	enc, err := datastore.NewVersionedInstanceParams(
		datastore.DefaultInstanceParamsVersion,
		lastFinalized.ID(),
		lastSealed.ID(),
		root.Params().SporkID(),
	)
	if err != nil {
		return fmt.Errorf("could not create versioned instance params: %w", err)
	}

	return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err = operation.InsertInstanceParams(lctx, rw, *enc)
		if err != nil {
			return fmt.Errorf("could not store instance params: %w", err)
		}

		// find the finalized seal that seals the lastSealed block, meaning seal.BlockID == lastSealed.ID()
		seal, err := segment.FinalizedSeal()
		if err != nil {
			return fmt.Errorf("could not get finalized seal from sealing segment: %w", err)
		}

		// Per convention, all blocks in the sealing segment must be finalized. Therefore, a QC must
		// exist for the `lastFinalized` block in the sealing segment. The QC for `lastFinalized` should be
		// contained in the `root` Snapshot and returned by `root.QuorumCertificate()`. Otherwise,
		// the Snapshot is incomplete, because consensus nodes require this QC. To reduce the chance of
		// accidental misconfiguration undermining consensus liveness, we do the following sanity checks:
		//  * `qcForLatestFinalizedBlock` should not be nil
		//  * `qcForLatestFinalizedBlock` should be for `lastFinalized` block, i.e. its view and blockID should match
		qcForLatestFinalizedBlock, err := root.QuorumCertificate()
		if err != nil {
			return fmt.Errorf("failed to obtain QC for latest finalized block from root sanpshot: %w", err)
		}
		if qcForLatestFinalizedBlock == nil {
			return fmt.Errorf("QC for latest finalized block in sealing segment cannot be nil")
		}
		if qcForLatestFinalizedBlock.BlockID != lastFinalized.ID() || qcForLatestFinalizedBlock.View != lastFinalized.View {
			return fmt.Errorf("latest finalized block from sealing segment (id %v, view=%d) does not match the root snapshot's tailing QC (certifying block %v with view %d)",
				lastFinalized.ID(), lastFinalized.View, qcForLatestFinalizedBlock.BlockID, qcForLatestFinalizedBlock.View)
		}

		// By definition, the root block / genesis block is the block with the lowest height and view. In other words, the latest
		// finalized block's view must be equal or greater than the view of the spork root block. We sanity check this relationship here:
		sporkRootBlockView := root.Params().SporkRootBlockView()
		if !(sporkRootBlockView <= lastFinalized.View) {
			return fmt.Errorf("sealing segment is invalid, because the latest finalized block's view %d is lower than the spork root block's view %d", lastFinalized.View, sporkRootBlockView)
		}
		safetyData := &hotstuff.SafetyData{
			LockedOneChainView:      lastFinalized.View,
			HighestAcknowledgedView: lastFinalized.View,
		}

		// We are given a QC for the latest finalized block, which proves that the view of the latest finalized block has been completed.
		// Hence, a freshly-bootstrapped consensus participant continues from the next view. Note that this guarantees that we are starting
		// in a view strictly greater than the spork root block's view, which is important for safety and liveness.
		livenessData := &hotstuff.LivenessData{
			CurrentView: lastFinalized.View + 1,
			NewestQC:    qcForLatestFinalizedBlock,
		}

		// persist safety and liveness data plus the QuorumCertificate for the latest finalized block for HotStuff/Jolteon consensus
		err = operation.UpsertSafetyData(lctx, rw, lastFinalized.ChainID, safetyData)
		if err != nil {
			return fmt.Errorf("could not insert safety data: %w", err)
		}
		err = operation.UpsertLivenessData(lctx, rw, lastFinalized.ChainID, livenessData)
		if err != nil {
			return fmt.Errorf("could not insert liveness data: %w", err)
		}
		err = operation.InsertQuorumCertificate(lctx, rw, qcForLatestFinalizedBlock)
		if err != nil {
			return fmt.Errorf("could not insert quorum certificate for the latest finalized block: %w", err)
		}

		w := rw.Writer()
		// insert height pointers
		err = operation.UpsertFinalizedHeight(lctx, w, lastFinalized.Height)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.UpsertSealedHeight(lctx, w, lastSealed.Height)
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
	})
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

// indexEpochHeights populates the epoch height index from the root snapshot.
// We index the FirstHeight for every epoch where the transition occurs within the sealing segment of the root snapshot,
// or for the first epoch of a spork if the snapshot is a spork root snapshot (1 block sealing segment).
//
// Caller must hold [storage.LockFinalizeBlock] lock.
//
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
	instanceParams, err := datastore.ReadInstanceParams(db.Reader(), headers, seals, blocks)
	if err != nil {
		return nil, fmt.Errorf("could not read instance params: %w", err)
	}
	sporkRootBlock := instanceParams.SporkRootBlock()

	globalParams := inmem.NewParams(
		inmem.EncodableParams{
			ChainID:              sporkRootBlock.ChainID,
			SporkID:              sporkRootBlock.ID(),
			SporkRootBlockHeight: sporkRootBlock.Height,
			SporkRootBlockView:   sporkRootBlock.View,
		},
	)
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

	// report information about latest known finalized block
	finalSnapshot := state.Final()
	latestFinalizedHeader, err := finalSnapshot.Head()
	if err != nil {
		return nil, fmt.Errorf("unexpected error to get finalized block: %w", err)
	}
	latestFinalizedBlock, err := state.blocks.ByHeight(latestFinalizedHeader.Height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve the latest sealed block by height: %w", err)
	}
	metrics.FinalizedHeight(latestFinalizedHeader.Height)
	metrics.BlockFinalized(latestFinalizedBlock)

	// report information about latest known finalized block
	latestSealedHeader, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get latest sealed block header: %w", err)
	}
	latestSealedBlock, err := state.blocks.ByHeight(latestSealedHeader.Height)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve the latest sealed block by height: %w", err)
	}
	metrics.SealedHeight(latestSealedHeader.Height)
	metrics.BlockSealed(latestSealedBlock)

	// report information about latest known epoch
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
// No errors expected during normal operation. Any error is a symptom of a bug or state corruption.
func IsBootstrapped(db storage.DB) (bool, error) {
	var finalized uint64
	err := operation.RetrieveFinalizedHeight(db.Reader(), &finalized)
	if errors.Is(err, operation.IncompleteStateError) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}

// GetChainID retrieves the consensus chainID from the latest finalized block in the database. This
// function reads directly from the database, without instantiating high-level storage abstractions
// or the protocol state struct.
//
// During bootstrapping, the latest finalized block and its height are indexed and thereafter the
// latest finalized height is only updated (but never removed). Hence, for a properly bootstrapped node,
// this function should _always_ return a proper value (constant throughout the lifetime of the node).
//
// Note: This function should only be called on properly bootstrapped nodes. If the state is corrupted
// or the node is not properly bootstrapped, this function may return [operation.IncompleteStateError].
// The reason for not returning [storage.ErrNotFound] directly is to avoid confusion between an often
// benign [storage.ErrNotFound] and failed reads of quantities that the protocol mandates to be present.
//
// No error returns expected during normal operations.
func GetChainID(db storage.DB) (flow.ChainID, error) {
	h, err := GetLatestFinalizedHeader(db)
	if err != nil {
		return "", fmt.Errorf("failed to determine chain ID: %w", err)
	}
	return h.ChainID, nil
}

// GetLatestFinalizedHeader retrieves the header of the latest finalized block. This function reads directly
// from the database, without instantiating high-level storage abstractions or the protocol state struct.
//
// During bootstrapping, the latest finalized block and its height are indexed and thereafter the latest
// finalized height is only updated (but never removed). Hence, for a properly bootstrapped node, this
// function should _always_ return a proper value.
//
// Note: This function should only be called on properly bootstrapped nodes. If the state is corrupted
// or the node is not properly bootstrapped, this function may return [operation.IncompleteStateError].
// The reason for not returning [storage.ErrNotFound] directly is to avoid confusion between an often
// benign [storage.ErrNotFound] and failed reads of quantities that the protocol mandates to be present.
//
// No error returns are expected during normal operations.
func GetLatestFinalizedHeader(db storage.DB) (*flow.Header, error) {
	var finalized uint64
	r := db.Reader()
	err := operation.RetrieveFinalizedHeight(r, &finalized)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve latest finalized height: %w", err)
	}
	var id flow.Identifier
	err = operation.LookupBlockHeight(r, finalized, &id)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve blockID of finalized block at height %d: %w", finalized, operation.IncompleteStateError)
	}
	var header flow.Header
	err = operation.RetrieveHeader(r, id, &header)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve latest finalized block %x: %w", id, operation.IncompleteStateError)
	}
	return &header, nil
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
func boostrapVersionBeacon(db storage.DB, snapshot protocol.Snapshot) error {
	versionBeacon, err := snapshot.VersionBeacon()
	if err != nil {
		return err
	}
	if versionBeacon == nil {
		return nil
	}
	return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.IndexVersionBeaconByHeight(rw.Writer(), versionBeacon)
	})
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
