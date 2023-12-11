package badger

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	statepkg "github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// cachedHeader caches a block header and its ID.
type cachedHeader struct {
	id     flow.Identifier
	header *flow.Header
}

type State struct {
	metrics module.ComplianceMetrics
	db      *badger.DB
	headers storage.Headers
	blocks  storage.Blocks
	qcs     storage.QuorumCertificates
	results storage.ExecutionResults
	seals   storage.Seals
	epoch   struct {
		setups  storage.EpochSetups
		commits storage.EpochCommits
	}
	globalParams             protocol.GlobalParams
	protocolStateSnapshotsDB storage.ProtocolState
	protocolState            protocol.MutableProtocolState
	versionBeacons           storage.VersionBeacons

	// rootHeight marks the cutoff of the history this node knows about. We cache it in the state
	// because it cannot change over the lifecycle of a protocol state instance. It is frequently
	// larger than the height of the root block of the spork, (also cached below as
	// `sporkRootBlockHeight`), for instance if the node joined in an epoch after the last spork.
	finalizedRootHeight uint64
	// sealedRootHeight returns the root block that is sealed.
	sealedRootHeight uint64
	// sporkRootBlockHeight is the height of the root block in the current spork. We cache it in
	// the state, because it cannot change over the lifecycle of a protocol state instance.
	// Caution: A node that joined in a later epoch past the spork, the node will likely _not_
	// know the spork's root block in full (though it will always know the height).
	sporkRootBlockHeight uint64
	// cache the latest finalized and sealed block headers as these are common queries.
	// It can be cached because the protocol state is solely responsible for updating these values.
	cachedFinal  *atomic.Pointer[cachedHeader]
	cachedSealed *atomic.Pointer[cachedHeader]
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
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateSnapshotsDB storage.ProtocolState,
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

	state := newState(
		metrics,
		db,
		headers,
		seals,
		results,
		blocks,
		qcs,
		setups,
		commits,
		protocolStateSnapshotsDB,
		versionBeacons,
		root.Params(),
	)

	if err := IsValidRootSnapshot(root, !config.SkipNetworkAddressValidation); err != nil {
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

	err = operation.RetryOnConflictTx(db, transaction.Update, func(tx *transaction.Tx) error {
		// sealing segment is in ascending height order, so the tail is the
		// oldest ancestor and head is the newest child in the segment
		// TAIL <- ... <- HEAD
		lastFinalized := segment.Finalized() // the highest block in sealing segment is the last finalized block
		lastSealed := segment.Sealed()       // the lowest block in sealing segment is the last sealed block

		// 1) bootstrap the sealing segment
		// creating sealed root block with the rootResult
		// creating finalized root block with lastFinalized
		err = state.bootstrapSealingSegment(segment, lastFinalized, rootSeal)(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap sealing chain segment blocks: %w", err)
		}

		// 2) insert the root quorum certificate into the database
		qc, err := root.QuorumCertificate()
		if err != nil {
			return fmt.Errorf("could not get root qc: %w", err)
		}
		err = qcs.StoreTx(qc)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root qc: %w", err)
		}

		// 3) initialize the current protocol state height/view pointers
		err = transaction.WithTx(state.bootstrapStatePointers(root))(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap height/view pointers: %w", err)
		}

		// 4) initialize values related to the epoch logic
		rootProtocolState, err := root.ProtocolState()
		if err != nil {
			return fmt.Errorf("could not retrieve protocol state for root snapshot: %w", err)
		}
		err = state.bootstrapEpoch(rootProtocolState, !config.SkipNetworkAddressValidation)(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap epoch values: %w", err)
		}

		// 5) initialize spork params
		err = transaction.WithTx(state.bootstrapSporkInfo(root))(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap spork info: %w", err)
		}

		// 6) bootstrap dynamic protocol state
		err = state.bootstrapProtocolState(segment, root, protocolStateSnapshotsDB)(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap protocol state: %w", err)
		}

		// 7) initialize version beacon
		err = transaction.WithTx(state.boostrapVersionBeacon(root))(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap version beacon: %w", err)
		}

		// 8) set metric values
		err = state.updateEpochMetrics(root)
		if err != nil {
			return fmt.Errorf("could not update epoch metrics: %w", err)
		}
		state.metrics.BlockSealed(lastSealed)
		state.metrics.SealedHeight(lastSealed.Header.Height)
		state.metrics.FinalizedHeight(lastFinalized.Header.Height)
		for _, block := range segment.Blocks {
			state.metrics.BlockFinalized(block)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed: %w", err)
	}

	// populate the protocol state cache
	err = state.populateCache()
	if err != nil {
		return nil, fmt.Errorf("failed to populate cache: %w", err)
	}

	return state, nil
}

// bootstrapProtocolState bootstraps data structures needed for Dynamic Protocol State.
// It inserts the root protocol state and indexes all blocks in the sealing segment assuming that
// dynamic protocol state didn't change in the sealing segment.
// The root snapshot's sealing segment must not straddle any epoch transitions
// or epoch phase transitions.
func (state *State) bootstrapProtocolState(segment *flow.SealingSegment, root protocol.Snapshot, protocolState storage.ProtocolState) func(tx *transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		rootProtocolState, err := root.ProtocolState()
		if err != nil {
			return fmt.Errorf("could not get root protocol state: %w", err)
		}
		rootProtocolStateEntry := rootProtocolState.Entry().ProtocolStateEntry
		protocolStateID := rootProtocolStateEntry.ID()
		err = protocolState.StoreTx(protocolStateID, rootProtocolStateEntry)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root protocol state: %w", err)
		}

		// NOTE: as specified in the godoc, this code assumes that each block
		// in the sealing segment in within the same phase within the same epoch.
		// the sealing segment.
		for _, block := range segment.AllBlocks() {
			err = protocolState.Index(block.ID(), protocolStateID)(tx)
			if err != nil {
				return fmt.Errorf("could not index root protocol state: %w", err)
			}
		}
		return nil
	}
}

// bootstrapSealingSegment inserts all blocks and associated metadata for the
// protocol state root snapshot to disk.
func (state *State) bootstrapSealingSegment(segment *flow.SealingSegment, head *flow.Block, rootSeal *flow.Seal) func(tx *transaction.Tx) error {
	return func(tx *transaction.Tx) error {

		for _, result := range segment.ExecutionResults {
			err := transaction.WithTx(operation.SkipDuplicates(operation.InsertExecutionResult(result)))(tx)
			if err != nil {
				return fmt.Errorf("could not insert execution result: %w", err)
			}
			err = transaction.WithTx(operation.IndexExecutionResult(result.BlockID, result.ID()))(tx)
			if err != nil {
				return fmt.Errorf("could not index execution result: %w", err)
			}
		}

		// insert the first seal (in case the segment's first block contains no seal)
		if segment.FirstSeal != nil {
			err := transaction.WithTx(operation.InsertSeal(segment.FirstSeal.ID(), segment.FirstSeal))(tx)
			if err != nil {
				return fmt.Errorf("could not insert first seal: %w", err)
			}
		}

		// root seal contains the result ID for the sealed root block. If the sealed root block is
		// different from the finalized root block, then it means the node dynamically bootstrapped.
		// In that case, we should index the result of the sealed root block so that the EN is able
		// to execute the next block.
		err := transaction.WithTx(operation.SkipDuplicates(operation.IndexExecutionResult(rootSeal.BlockID, rootSeal.ResultID)))(tx)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		for _, block := range segment.ExtraBlocks {
			blockID := block.ID()
			height := block.Header.Height
			err := state.blocks.StoreTx(block)(tx)
			if err != nil {
				return fmt.Errorf("could not insert SealingSegment extra block: %w", err)
			}
			err = transaction.WithTx(operation.IndexBlockHeight(height, blockID))(tx)
			if err != nil {
				return fmt.Errorf("could not index SealingSegment extra block (id=%x): %w", blockID, err)
			}
			err = state.qcs.StoreTx(block.Header.QuorumCertificate())(tx)
			if err != nil {
				return fmt.Errorf("could not store qc for SealingSegment extra block (id=%x): %w", blockID, err)
			}
		}

		for i, block := range segment.Blocks {
			blockID := block.ID()
			height := block.Header.Height

			err := state.blocks.StoreTx(block)(tx)
			if err != nil {
				return fmt.Errorf("could not insert SealingSegment block: %w", err)
			}
			err = transaction.WithTx(operation.IndexBlockHeight(height, blockID))(tx)
			if err != nil {
				return fmt.Errorf("could not index SealingSegment block (id=%x): %w", blockID, err)
			}
			err = state.qcs.StoreTx(block.Header.QuorumCertificate())(tx)
			if err != nil {
				return fmt.Errorf("could not store qc for SealingSegment block (id=%x): %w", blockID, err)
			}

			// index the latest seal as of this block
			latestSealID, ok := segment.LatestSeals[blockID]
			if !ok {
				return fmt.Errorf("missing latest seal for sealing segment block (id=%s)", blockID)
			}
			// sanity check: make sure the seal exists
			var latestSeal flow.Seal
			err = transaction.WithTx(operation.RetrieveSeal(latestSealID, &latestSeal))(tx)
			if err != nil {
				return fmt.Errorf("could not verify latest seal for block (id=%x) exists: %w", blockID, err)
			}
			err = transaction.WithTx(operation.IndexLatestSealAtBlock(blockID, latestSealID))(tx)
			if err != nil {
				return fmt.Errorf("could not index block seal: %w", err)
			}

			// for all but the first block in the segment, index the parent->child relationship
			if i > 0 {
				err = transaction.WithTx(operation.InsertBlockChildren(block.Header.ParentID, []flow.Identifier{blockID}))(tx)
				if err != nil {
					return fmt.Errorf("could not insert child index for block (id=%x): %w", blockID, err)
				}
			}
		}

		// insert an empty child index for the final block in the segment
		err = transaction.WithTx(operation.InsertBlockChildren(head.ID(), nil))(tx)
		if err != nil {
			return fmt.Errorf("could not insert child index for head block (id=%x): %w", head.ID(), err)
		}

		return nil
	}
}

// bootstrapStatePointers instantiates special pointers used to by the protocol
// state to keep track of special block heights and views.
func (state *State) bootstrapStatePointers(root protocol.Snapshot) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		segment, err := root.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}
		highest := segment.Finalized()
		lowest := segment.Sealed()
		// find the finalized seal that seals the lowest block, meaning seal.BlockID == lowest.ID()
		seal, err := segment.FinalizedSeal()
		if err != nil {
			return fmt.Errorf("could not get finalized seal from sealing segment: %w", err)
		}

		safetyData := &hotstuff.SafetyData{
			LockedOneChainView:      highest.Header.View,
			HighestAcknowledgedView: highest.Header.View,
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
		if rootQC.View != highest.Header.View {
			return fmt.Errorf("root QC's view %d does not match the highest block in sealing segment (view %d)", rootQC.View, highest.Header.View)
		}
		if rootQC.BlockID != highest.Header.ID() {
			return fmt.Errorf("root QC is for block %v, which does not match the highest block %v in sealing segment", rootQC.BlockID, highest.Header.ID())
		}

		livenessData := &hotstuff.LivenessData{
			CurrentView: highest.Header.View + 1,
			NewestQC:    rootQC,
		}

		// insert initial views for HotStuff
		err = operation.InsertSafetyData(highest.Header.ChainID, safetyData)(tx)
		if err != nil {
			return fmt.Errorf("could not insert safety data: %w", err)
		}
		err = operation.InsertLivenessData(highest.Header.ChainID, livenessData)(tx)
		if err != nil {
			return fmt.Errorf("could not insert liveness data: %w", err)
		}

		// insert height pointers
		err = operation.InsertRootHeight(highest.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized root height: %w", err)
		}
		// the sealed root height is the lowest block in sealing segment
		err = operation.InsertSealedRootHeight(lowest.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed root height: %w", err)
		}
		err = operation.InsertFinalizedHeight(highest.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(lowest.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}
		err = operation.IndexFinalizedSealByBlockID(seal.BlockID, seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index sealed block: %w", err)
		}

		// insert first-height indices for epochs which have started
		hasPrevious, err := protocol.PreviousEpochExists(root)
		if err != nil {
			return fmt.Errorf("could not check existence of previous epoch: %w", err)
		}
		if hasPrevious {
			err = indexFirstHeight(root.Epochs().Previous())(tx)
			if err != nil {
				return fmt.Errorf("could not index previous epoch first height: %w", err)
			}
		}
		err = indexFirstHeight(root.Epochs().Current())(tx)
		if err != nil {
			return fmt.Errorf("could not index current epoch first height: %w", err)
		}

		return nil
	}
}

// bootstrapEpoch bootstraps the protocol state database with information about
// the previous, current, and next epochs as of the root snapshot.
func (state *State) bootstrapEpoch(rootProtocolState protocol.DynamicProtocolState, verifyNetworkAddress bool) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		richEntry := rootProtocolState.Entry()

		// keep track of EpochSetup/EpochCommit service events, then store them after this step is complete
		var setups []*flow.EpochSetup
		var commits []*flow.EpochCommit

		// validate and insert previous epoch if it exists
		if rootProtocolState.PreviousEpochExists() {
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

		// validate and insert current epoch
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
					return fmt.Errorf("invalid EpochCommit for next epoch")
				}
				commits = append(commits, commit)
			}
		}

		// insert all epoch setup/commit service events
		// dynamic protocol state relies on these events being stored
		for _, setup := range setups {
			err := state.epoch.setups.StoreTx(setup)(tx)
			if err != nil {
				return fmt.Errorf("could not store epoch setup event: %w", err)
			}
		}
		for _, commit := range commits {
			err := state.epoch.commits.StoreTx(commit)(tx)
			if err != nil {
				return fmt.Errorf("could not store epoch commit event: %w", err)
			}
		}

		return nil
	}
}

// bootstrapSporkInfo bootstraps the protocol state with information about the
// spork which is used to disambiguate Flow networks.
func (state *State) bootstrapSporkInfo(root protocol.Snapshot) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		params := root.Params()

		sporkID := params.SporkID()
		err := operation.InsertSporkID(sporkID)(tx)
		if err != nil {
			return fmt.Errorf("could not insert spork ID: %w", err)
		}

		sporkRootBlockHeight := params.SporkRootBlockHeight()
		err = operation.InsertSporkRootBlockHeight(sporkRootBlockHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not insert spork root block height: %w", err)
		}

		version := params.ProtocolVersion()
		err = operation.InsertProtocolVersion(version)(tx)
		if err != nil {
			return fmt.Errorf("could not insert protocol version: %w", err)
		}

		threshold := params.EpochCommitSafetyThreshold()
		err = operation.InsertEpochCommitSafetyThreshold(threshold)(tx)
		if err != nil {
			return fmt.Errorf("could not insert epoch commit safety threshold: %w", err)
		}

		return nil
	}
}

// indexFirstHeight indexes the first height for the epoch, as part of bootstrapping.
// The input epoch must have been started (the first block of the epoch has been finalized).
// No errors are expected during normal operation.
func indexFirstHeight(epoch protocol.Epoch) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		counter, err := epoch.Counter()
		if err != nil {
			return fmt.Errorf("could not get epoch counter: %w", err)
		}
		firstHeight, err := epoch.FirstHeight()
		if err != nil {
			return fmt.Errorf("could not get epoch first height: %w", err)
		}
		err = operation.InsertEpochFirstHeight(counter, firstHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not index first height %d for epoch %d: %w", firstHeight, counter, err)
		}
		return nil
	}
}

func OpenState(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolState storage.ProtocolState,
	versionBeacons storage.VersionBeacons,
) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	globalParams, err := ReadGlobalParams(db, headers)
	if err != nil {
		return nil, fmt.Errorf("could not read global params")
	}
	state := newState(
		metrics,
		db,
		headers,
		seals,
		results,
		blocks,
		qcs,
		setups,
		commits,
		protocolState,
		versionBeacons,
		globalParams,
	) // populate the protocol state cache
	err = state.populateCache()
	if err != nil {
		return nil, fmt.Errorf("failed to populate cache: %w", err)
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

	// update all epoch related metrics
	err = state.updateEpochMetrics(finalSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to update epoch metrics: %w", err)
	}

	return state, nil
}

func (state *State) Params() protocol.Params {
	return Params{
		GlobalParams:   state.globalParams,
		InstanceParams: &InstanceParams{state: state},
	}
}

// Sealed returns a snapshot for the latest sealed block. A latest sealed block
// must always exist, so this function always returns a valid snapshot.
func (state *State) Sealed() protocol.Snapshot {
	cached := state.cachedSealed.Load()
	if cached == nil {
		return invalid.NewSnapshotf("internal inconsistency: no cached sealed header")
	}
	return NewFinalizedSnapshot(state, cached.id, cached.header)
}

// Final returns a snapshot for the latest finalized block. A latest finalized
// block must always exist, so this function always returns a valid snapshot.
func (state *State) Final() protocol.Snapshot {
	cached := state.cachedFinal.Load()
	if cached == nil {
		return invalid.NewSnapshotf("internal inconsistency: no cached final header")
	}
	return NewFinalizedSnapshot(state, cached.id, cached.header)
}

// AtHeight returns a snapshot for the finalized block at the given height.
// This function may return an invalid.Snapshot with:
//   - state.ErrUnknownSnapshotReference:
//     -> if no block with the given height has been finalized, even if it is incorporated
//     -> if the given height is below the root height
//   - exception for critical unexpected storage errors
func (state *State) AtHeight(height uint64) protocol.Snapshot {
	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := state.db.View(operation.LookupBlockHeight(height, &blockID))
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
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	qcs storage.QuorumCertificates,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateSnapshots storage.ProtocolState,
	versionBeacons storage.VersionBeacons,
	globalParams protocol.GlobalParams,
) *State {
	state := &State{
		metrics: metrics,
		db:      db,
		headers: headers,
		results: results,
		seals:   seals,
		blocks:  blocks,
		qcs:     qcs,
		epoch: struct {
			setups  storage.EpochSetups
			commits storage.EpochCommits
		}{
			setups:  setups,
			commits: commits,
		},
		globalParams:             globalParams,
		protocolStateSnapshotsDB: protocolStateSnapshots,
		protocolState: protocol_state.NewMutableProtocolState(
			protocolStateSnapshots,
			globalParams,
			headers,
			results,
			setups,
			commits,
		),
		versionBeacons: versionBeacons,
		cachedFinal:    new(atomic.Pointer[cachedHeader]),
		cachedSealed:   new(atomic.Pointer[cachedHeader]),
	}

	return state
}

// IsBootstrapped returns whether the database contains a bootstrapped state
func IsBootstrapped(db *badger.DB) (bool, error) {
	var finalized uint64
	err := db.View(operation.RetrieveFinalizedHeight(&finalized))
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
func (state *State) updateEpochMetrics(snap protocol.Snapshot) error {

	// update epoch counter
	counter, err := snap.Epochs().Current().Counter()
	if err != nil {
		return fmt.Errorf("could not get current epoch counter: %w", err)
	}
	state.metrics.CurrentEpochCounter(counter)

	// update epoch phase
	phase, err := snap.Phase()
	if err != nil {
		return fmt.Errorf("could not get current epoch counter: %w", err)
	}
	state.metrics.CurrentEpochPhase(phase)

	currentEpochFinalView, err := snap.Epochs().Current().FinalView()
	if err != nil {
		return fmt.Errorf("could not update current epoch final view: %w", err)
	}
	state.metrics.CurrentEpochFinalView(currentEpochFinalView)

	dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := protocol.DKGPhaseViews(snap.Epochs().Current())
	if err != nil {
		return fmt.Errorf("could not get dkg phase final view: %w", err)
	}

	state.metrics.CurrentDKGPhase1FinalView(dkgPhase1FinalView)
	state.metrics.CurrentDKGPhase2FinalView(dkgPhase2FinalView)
	state.metrics.CurrentDKGPhase3FinalView(dkgPhase3FinalView)

	// EECC - check whether the epoch emergency fallback flag has been set
	// in the database. If so, skip updating any epoch-related metrics.
	epochFallbackTriggered, err := state.isEpochEmergencyFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check epoch emergency fallback flag: %w", err)
	}
	if epochFallbackTriggered {
		state.metrics.EpochEmergencyFallbackTriggered()
	}

	return nil
}

// boostrapVersionBeacon bootstraps version beacon, by adding the latest beacon
// to an index, if present.
func (state *State) boostrapVersionBeacon(
	snapshot protocol.Snapshot,
) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		versionBeacon, err := snapshot.VersionBeacon()
		if err != nil {
			return err
		}

		if versionBeacon == nil {
			return nil
		}

		return operation.IndexVersionBeaconByHeight(versionBeacon)(txn)
	}
}

// populateCache is used after opening or bootstrapping the state to populate the cache.
// The cache must be populated before the State receives any queries.
// No errors expected during normal operations.
func (state *State) populateCache() error {

	// cache the initial value for finalized block
	err := state.db.View(func(tx *badger.Txn) error {
		// root height
		err := state.db.View(operation.RetrieveRootHeight(&state.finalizedRootHeight))
		if err != nil {
			return fmt.Errorf("could not read root block to populate cache: %w", err)
		}
		// sealed root height
		err = state.db.View(operation.RetrieveSealedRootHeight(&state.sealedRootHeight))
		if err != nil {
			return fmt.Errorf("could not read sealed root block to populate cache: %w", err)
		}
		// spork root block height
		err = state.db.View(operation.RetrieveSporkRootBlockHeight(&state.sporkRootBlockHeight))
		if err != nil {
			return fmt.Errorf("could not get spork root block height: %w", err)
		}
		// finalized header
		var finalizedHeight uint64
		err = operation.RetrieveFinalizedHeight(&finalizedHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup finalized height: %w", err)
		}
		var cachedFinalHeader cachedHeader
		err = operation.LookupBlockHeight(finalizedHeight, &cachedFinalHeader.id)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup finalized id (height=%d): %w", finalizedHeight, err)
		}
		cachedFinalHeader.header, err = state.headers.ByBlockID(cachedFinalHeader.id)
		if err != nil {
			return fmt.Errorf("could not get finalized block (id=%x): %w", cachedFinalHeader.id, err)
		}
		state.cachedFinal.Store(&cachedFinalHeader)
		// sealed header
		var sealedHeight uint64
		err = operation.RetrieveSealedHeight(&sealedHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup sealed height: %w", err)
		}
		var cachedSealedHeader cachedHeader
		err = operation.LookupBlockHeight(sealedHeight, &cachedSealedHeader.id)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup sealed id (height=%d): %w", sealedHeight, err)
		}
		cachedSealedHeader.header, err = state.headers.ByBlockID(cachedSealedHeader.id)
		if err != nil {
			return fmt.Errorf("could not get sealed block (id=%x): %w", cachedSealedHeader.id, err)
		}
		state.cachedSealed.Store(&cachedSealedHeader)
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not cache finalized header: %w", err)
	}

	return nil
}

// isEpochEmergencyFallbackTriggered checks whether epoch fallback has been globally triggered.
// Returns:
// * (true, nil) if epoch fallback is triggered
// * (false, nil) if epoch fallback is not triggered (including if the flag is not set)
// * (false, err) if an unexpected error occurs
func (state *State) isEpochEmergencyFallbackTriggered() (bool, error) {
	var triggered bool
	err := state.db.View(operation.CheckEpochEmergencyFallbackTriggered(&triggered))
	return triggered, err
}
