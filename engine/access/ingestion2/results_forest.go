package ingestion2

import (
	"fmt"
	"iter"
	"sort"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/storage"
)

var (
	// ErrMaxViewDeltaExceeded is returned when attempting to add a result whose view is
	// more than maxViewDelta views ahead of the last sealed and persisted result. In the
	// context of the ResultsForest, the view of a result is the view of the executed block.
	ErrMaxViewDeltaExceeded = fmt.Errorf("result's block is outside view range currenlty covered by ResultForest")
)

// ResultsForest is a mempool holding execution results (and receipts), which is aware of the tree
// structure formed by the results. The mempool provides functional primitives for deciding if and when
// data for an execution result should be downloaded, processed, persisted, and/or abandoned.
//
// Usage pattern:
//   - The ResultsForest serves as a stateful, fork-aware mempool, to temporarily store the progress of ingesting
//     different execution results and their receipts.
//   - The ResultsForest is intended to 𝙨𝙩𝙤𝙧𝙚 𝙖𝙡𝙡 𝙠𝙣𝙤𝙬𝙣 𝙧𝙚𝙨𝙪𝙡𝙩𝙨 𝙬𝙞𝙩𝙝𝙞𝙣 𝙖 𝙬𝙞𝙣𝙙𝙤𝙬 𝙤𝙛 𝙫𝙞𝙚𝙬𝙨. It only works for use cases, where
//     processing of results with lower views takes precedence over results with higher views (only exception being
//     execution forks that are known to be orphaned).
//   - Results are accepted by the mempool if and only if they fall within the ResultsForest's current view window;
//     it is *not* necessary for all ancestor results to be present in the forest first.
//   - However, for its liveness, the ResultsForest requires that every result sealed by the protocol is 𝙚𝙫𝙚𝙣𝙩𝙪𝙖𝙡𝙡𝙮
//     added to the ResultsForest successfully (either as wrapped inside an Execution Node's receipt or as a
//     stand-alone sealed result).
//   - The ResultsForest mempool encapsulates the logic for maintaining a forest of execution results, including
//     the receipts and processing status of each result. While it is concurrency safe, it utilizes the calling
//     threads to execute its internal logic.
//
// Nomenclature:
//   - Within the context of the ResultsForest, we refer to a 𝒓𝒆𝒔𝒖𝒍𝒕 as 𝒔𝒆𝒂𝒍𝒆𝒅, if and only if a seal for the result
//     exists in a finalized block.
//   - Within the scope of the ResultsForest, the 𝙫𝙞𝙚𝙬 of a result is defined as the view of the executed block.
//     We denote the view of a result 𝒓 as `𝒓.𝙇𝙚𝙫𝙚𝙡` (adopting the more generic terminology of the LevelledForest).
//   - 𝘼𝙣𝙘𝙚𝙨𝙩𝙤𝙧 is a transitive binary relation between two results 𝒓₁ and 𝒓₂. We say that 𝒓₁ is an ancestor of 𝒓₂
//     if 𝒓₁ can be reached from 𝒓₂ following the `PreviousResultID` fields of the results, only using results that
//     are stored in the ResultsForest. 𝒓₁ is the parent of 𝒓₂, iff 𝒓₂.PreviousResultID == 𝒓₁.ID(); in this case, we
//     say that 𝒓₁ is the ancestor of degree 1 of 𝒓₂. The grandparent is the ancestor of degree 2, etc. Lastly, a
//     result is its own ancestor of degree 0.
//
// Conceptually, the ResultsForest maintains the following three quantities:
//  1. 𝓹 tracks the 𝙡𝙤𝙬𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝘄𝗶𝘁𝗵𝗶𝗻 the ResultsForest. Specifically, we require that
//     (i) 𝓹 is sealed (a seal for it has been included in a finalized block) and
//     (ii) no results with a lower view exist in the forest.
//  2. 𝓼 is a local notion of the 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝘄𝗶𝘁𝗵𝗶𝗻 𝘁𝗵𝗲 𝗳𝗼𝗿𝗲𝘀𝘁. Specifically, we require all
//     of the following attributes to hold:
//     (i) 𝓼 is sealed (a seal for it has been included in a finalized block).
//     (ii) All parent and ancestor results exist in the forest up to and including the pruning-threshold view.
//     Specifically, recursing the execution fork from 𝓼 backwards following the `PreviousResultID`, we
//     eventually will reach 𝓹.
//     (iii) No other result 𝒓 resists in the forest that satisfies (i) and (ii) but has a higher view than 𝓼.
//     Note that this definition purposefully excludes results that have been sealed by the consensus nodes,
//     but which the ResultsForest hasn't ingested yet or where some ancestor results are not yet available.
//  3. 𝓱 is the ResultsForest's 𝙫𝙞𝙚𝙬 𝙝𝙤𝙧𝙞𝙯𝙤𝙣. No results with larger view exist in the forest.
//  4. `rejectedResults` is a boolean value that indicates whether the ResultsForest has rejected any results.
//     During instantiation, it is initialized to false. It is set to true if and only if the ResultsForest
//     rejects a result with view > 𝓱. The ResultsForest allows external business logic to reset
//     `rejectedResults` back to false, by calling `ResetLowestRejectedView` (details below).
//
// At runtime, the ResultsForest enforces that 𝓹, 𝓼, 𝓱 always satisfy the following relationships referred
// to collectively as 𝙞𝙣𝙫𝙖𝙧𝙞𝙖𝙣𝙩. The invariant is a necessary condition for correct protocol execution and a
// sufficient condition for liveness of data ingestion.
//   - 𝓹 and 𝓼 always exist in the forest
//   - 𝓹 is an ancestor of 𝓼. We allow the degenerate case, where 𝓹 is 𝓼's ancestor of degree zero, i.e.
//     𝓹 and 𝓼 reference the same result. Consequently, 𝓹's view must be smaller or equal to the view of
//     𝓼; formally 𝓹.Level ≤ 𝓼.Level.
//   - 𝓹.Level ≤ 𝓼.Level ≤ 𝓱
//     with the additional constraint that 𝓹.Level < 𝓱 (required for liveness)
//   - 𝓹, 𝓼, 𝓱, monotonically increase during the runtime of the ResultsForest
//
// Any honest protocol execution should satisfy the invariant. Hence, the invariant being violated is a
// symptom of a severe bug in the protocol implementation or a corrupted internal state. Either way, safe
// continuation is not possible and the node should restart.
// With the following additional 𝒄𝒐𝒏𝒕𝒓𝒂𝒄𝒕, the invariant is sufficient to guarantee safety and liveness
// of the ResultsForest (proof to be written up):
//
//   - The ResultsForest offers the method `ResetLowestRejectedView() 𝒔 uint64`, which resets
//     `rejectedResults = false` and returns the view 𝒔 := 𝓼.Level. When calling `ResetLowestRejectedView`,
//     the higher-level business logic promises that all sealed results with views ≥ 𝒔 will eventually be
//     provided to the ResultsForest. Sealed results must be provided, while unsealed results may be added.
//   - The higher-level business logic abides by this contract up to the point where the ResultsForest
//     rejects a result again later.
//   - When calling `ResetLowestRejectedView`, all prior contracts are annulled, and we engage again in the
//     contract of delivering all sealed results with views ≥ 𝒔, where 𝒔 denotes the value returned by the
//     most recent call to `ResetLowestRejectedView`.
//   - The higher-level business logic must perform the following backfill process:
//     (i) provide sealed results from the returned 𝓼.Level up to and including the latest sealed result
//     from the consensus follower.
//     (ii) inform the forest about sealing of results that are being backfilled.
//     This ensures that the forest is fully up to date with the higher-level business logic's view of the
//     protocol's global state such that seals within any new OnFinalizedBlock event extend from the
//     forest's latest sealed result 𝓼.
//
// Safety here means that only results marked as sealed by the protocol will be considered sealed by the ResultsForest.
// Liveness means that every result sealed by the protocol will eventually be considered as processed by the ResultsForest.
//
// Important:
//   - The ResultsForest is intended to run in an environment where the Consensus Follower ingests blocks. The
//     Follower only accepts blocks once they are certified (i.e. a QC exists for the block). Per guarantees of
//     the Jolteon consensus algorithm, among all blocks for some view, at most one can be certified.
//   - The ResultsForest is designed to be 𝙚𝙫𝙚𝙣𝙩𝙪𝙖𝙡𝙡𝙮 𝙘𝙤𝙣𝙨𝙞𝙨𝙩𝙚𝙣𝙩. This means that its local values for 𝓹, 𝓼, and 𝓱
//     may be lagging behind conceptually similar quantities in the Consensus Follower (or other components in
//     the node). This is an intentional design choice to increase modularity, reduce requirements on other
//     components, and allow components to progress independently. Overall, an eventually consistent design
//     helps to keep intellectual complexity of the protocol implementation manageable while also strengthening
//     BFT of the node implementations.
//   - 𝓹.Level essentially parameterizes the lower bound of the view window in which the ResultsForest accepts
//     results. Other components building on top of the ResultsForest may require a shorter history. This is
//     fine - the ResultsForest must maintain sufficient history to allow the higher-level components to work, but
//     it may temporarily have a longer history than strictly necessary, as long as it eventually prunes it.
//   - Similarly, 𝓼.Level is the view up to which the ResultsForest can track sealing progress. Though, the protocol
//     might already have sealed further blocks, some of which might not have been ingested by the ResultsForest yet.
//     This is another case, where the ResultsForest local notion lags behind the protocol's global view, which
//     is fine as long as the forest eventually receives the result and is told that it is sealed.
//   - The ResultsForest is an information-driven system and information is idempotent. Inputs are information about
//     the protocol's global view rather than commands for the ResultsForest to do a certain thing. As illustration,
//     consider a Alice walking up to a cliff. Telling Alice that "it is safe to walk up to 3m before the cliff"
//     would be the information-driven approach. Repeatedly giving the same information is safe. Also receiving outdated
//     information is safe - such as telling Alice that "it is safe to walk up to 5m before the cliff". Both
//     packages of information are consistent in that they declare 5m as a safe distance to the cliff; furthermore,
//     the first package of information provides additional insights: it is also safe to walk up to 3m to the cliff.
//     In contrast, the command "walk 1 meter forward" would not be idempotent, because accidental repetition causes
//     Alice to eventually fall off the cliff. The latter approach is safe only if the commander observing Alice's
//     distance to the cliff, issuing the command, and Alice executing the command all happens within one atomic operation.
//     Atomic state updates across different concurrent components are a major source of complexity. We reduce those
//     complexities by designing the ResultsForest as an information-driven, eventually consistent system.
//     Examples of Idempotent operations in the ResultsForest:
//   - Pruning: telling the ResultsForest that all results with views smaller 70 can be pruned and later
//     informing the forest that results with views smaller 60 can be pruned should be acceptable. The forest
//     may respond with a dedicated sentinel informing the caller that it already has pruned up to view 70.
//     Nevertheless, from the perspective of the ResultsForest, it should not be treated as a critical exception,
//     because the information that all blocks up to view 60 are allowed to be pruned is a subset of the information
//     the forest got before when being told that all blocks up to view 70 can be pruned.
//   - Repeated addition of the same result and/or receipt should be a no-op.
//   - <additional conditions?>
//
// It is the forest's responsibility to make progress up to its 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼.
// Per definition of 𝓼, the ancestry of 𝓼 is stored in the forest, so the forest has the means to
// progress up to 𝓼. If the forest satisfies this design decision (specific argument why this is
// the case for our implementation at had is still to be written down), the following statement is
// true:
//   - As long as the forest does not reject any results, the forest 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼 𝘄𝗶𝘁𝗵𝗶𝗻 𝘁𝗵𝗲 𝗳𝗼𝗿𝗲𝘀𝘁
//     will continue to grow, because the forest is being fed with results in ancestor-first order.
//     Hence, the indexing process is guaranteed to be live up to 𝓼.
//
// The Consensus follower guarantees that blocks are incorporated in "ancestor first order".
// Specifically that means that when the consensus follower ingests block B, it has previously
// incorporated block B.ParentID.
//   - By induction, this means that all blocks that the consensus follower declares as
//     "incorporated" (via OnBlockIncorporated notifications) descend from the root block the node
//     was boostrapped with.
//   - Furthermore, OnBlockIncorporated are delivered in an order-preserving manner. Meaning, a
//     component subscribed to the OnBlockIncorporated notifications also hears about blocks in an
//     ancestor-first order.
//   - Unless the node crashes, OnBlockIncorporated notifications are delivered to the subscribers for
//     every block that the consensus follower ingests.
//
// Consensus makes a similar guarantee for results included in blocks:
//   - Results are included into a fork in an ancestor-first ordering. Meaning, the proposer of
//     block B may include a result R only if the result referenced by R.PreviousResultID was
//     included in B or its ancestors.
//
// The result forest requires that finalized block and sealed result notifications are both delivered
// in ancestor first order. This is relaxed for unsealed results which may be delivered in any order.
//
// The ResultsForest mempool supports pruning by view:
// only results descending from the latest sealed and finalized result are relevant.
// By convention, the ResultsForest always contains the latest sealed result. Thereby, the
// ResultsForest is able to determine whether results for a block still need to be processed or
// can be orphaned (processing abandoned). Hence, we prune all results for blocks _below_ the
// latest block with a finalized seal, once it has been persisted in the database.
// All results for views at or above the pruning threshold are retained, explicitly including results
// from execution forks or orphaned blocks even if they conflict with the finalized seal. However, such
// orphaned forks will eventually stop growing, because either (i) a conflicting fork of blocks is
// finalized, which means that the orphaned forks can no longer be extended by new blocks or (ii) a
// rogue execution node pursuing its own execution fork will eventually be slashed and can no longer
// submit new results.
// Nevertheless, to utilize resources efficiently, the ResultsForest tries to avoid processing execution
// forks that conflict with the finalized seal.
//
// Pruning and abandoning processing of results:
//   - For liveness, the ResultsForest must guarantee that all sealed results it knows about are
//     eventually marked as processable.
//   - For results that are not yet sealed, only results descending from the latest sealed result 𝓼 should
//     ideally be processed. The ResultsForest is allowed to process results optimistically (resources
//     permitting), accepting the possibility that some results will turn out to be orphaned later.
//   - To utilize resources efficiently, the ResultsForest tries to avoid processing execution forks that
//     conflict with the finalized seal. Furthermore, it attempts to cancel already ongoing processing once
//     it concludes that a fork was abandoned. Specifically:
//     (a) processing of execution forks that conflict with known sealed results are be abandoned
//     (b) processing of execution forks whose executed blocks conflict with finalized blocks can be abandoned
//   - By convention, the ResultsForest always retains the 𝒍𝒂𝒕𝒆𝒔𝒕 𝒔𝒆𝒂𝒍𝒆𝒅 𝒂𝒏𝒅 𝒑𝒓𝒐𝒄𝒆𝒔𝒔𝒆𝒅 (denoted as 𝓹 above). Thereby,
//     the ResultsForest is able to determine whether results for a block still need to be processed or can be
//     orphaned (processing abandoned). Hence, we prune all results for blocks _below_ the latest block with
//     a finalized seal, once it has been persisted in the database.
//     All results for views at or above the pruning threshold are retained, explicitly including results
//     from execution forks or orphaned blocks even if they conflict with the finalized seal. Nevertheless,
//     all orphaned execution forks will eventually be dropped by the ResultsForest, as pruning progresses.
//     The reason is that orphaned execution forks will eventually stop growing, because either
//     (i) a conflicting fork of blocks is finalized, which means that the orphaned forks can no longer
//     be extended by new blocks or (ii) a rogue execution node pursuing its own execution fork will
//     eventually be slashed and can no longer submit new results.
//
// Finalized block tracking:
//   - The ResultsForest tracks the finalized block status of results within the forest. This is used
//     exclusively by higher-level business logic (e.g. ForestManager) to determine whether a result
//     can be processed.
//   - Since results for finalized blocks are not guaranteed by the protocol to exist before sealing,
//     the process used to guarantee a fully connected chain from 𝓹 to 𝓼 is not possible for finalized
//     results.
//   - Given this, the forest does not make any guarantees about the connectedness of finalized results.
//   - Since finalized blocks are guaranteed by the protocol to eventually be executed (except in rare
//     cases involving sporks), a result for the finalized block will eventually be sealed.
//
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type ResultsForest struct {
	log             zerolog.Logger
	forest          forest.LevelledForest
	headers         storage.Headers
	pipelineFactory optimistic_sync.PipelineFactory

	// lastSealedView is the view of the last sealed result.
	lastSealedView counters.StrictMonotonicCounter

	// lastFinalizedView is the view of the last finalized block processed by the forest.
	lastFinalizedView counters.StrictMonotonicCounter

	// latestPersistedSealedResult tracks metadata about the latest persisted sealed result.
	// this represents the lowest sealed result within the forest.
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader

	// maxViewDelta specifies the number of views past its lowest view that the forest will accept.
	// maxViewDelta is added to the lowest view to compute the view horizon.
	// Results for views higher than view horizon are rejected. This ensures that the forest does
	// not grow unbounded.
	maxViewDelta uint64

	// rejectedResults is a boolean value that indicates whether the ResultsForest has rejected any
	// results because their view exceeds the forest's view horizon.
	rejectedResults bool

	mu sync.RWMutex
}

// NewResultsForest creates a new instance of ResultsForest.
// No errors are expected during normal operations.
func NewResultsForest(
	log zerolog.Logger,
	headers storage.Headers,
	pipelineFactory optimistic_sync.PipelineFactory,
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader,
	maxViewDelta uint64,
) (*ResultsForest, error) {
	_, sealedHeight := latestPersistedSealedResult.Latest()
	sealedHeader, err := headers.ByHeight(sealedHeight) // by protocol convention, this should always exist (initialized during bootstrapping)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header for latest persisted sealed result (height: %d): %w", sealedHeight, err)
	}

	return &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest.NewLevelledForest(sealedHeader.View),
		headers:                     headers,
		pipelineFactory:             pipelineFactory,
		maxViewDelta:                maxViewDelta,
		lastSealedView:              counters.NewMonotonicCounter(sealedHeader.View),
		lastFinalizedView:           counters.NewMonotonicCounter(sealedHeader.View),
		latestPersistedSealedResult: latestPersistedSealedResult,
	}, nil
}

// ResetLowestRejectedView returns the last sealed view processed by the forest, and resets the
// rejected flag. If no results have been rejected since the last call, false is returned.
//
// This is used by higher-level business logic to implement the backfill process. The caller should:
//   - Call `ResetLowestRejectedView` to get the last sealed view processed by the forest.
//   - If false is returned, no results have been rejected so the forest is up to date with all
//     previously submitted results.
//   - Otherwise, the caller should resubmit all sealed results from the returned view up to and
//     including the latest sealed result from the consensus follower using `AddSealedResult`.
func (rf *ResultsForest) ResetLowestRejectedView() (uint64, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rejectedResults := rf.rejectedResults
	rf.rejectedResults = false

	return rf.lastSealedView.Value(), rejectedResults
}

// AddSealedResult adds a SEALED Execution Result to the Result Forest (without any receipts),
// in case the result is not already stored in the tree. If the result is already stored,
// its status is set to sealed.
//
// IMPORTANT: The caller MUST provide sealed results in ancestor first order to allow the forest to
// guarantee liveness. If a sealed result is provided who's parent is either missing or not sealed,
// an error is returned.
//
// Execution results with a finalized seal are committed to the chain and considered final
// irrespective of which Execution Nodes [ENs] produced them. Therefore, it is fine to not track
// which ENs produced the result (but we also don't preclude a receipt from being added for the
// result later).
//
// In contrast, for results without a finalized seal, it depends on the clients how much trust they
// are willing to place in the results correctness - potentially depending on how many ENs and/or
// which ENs specifically produced the result. Therefore, unsealed results must be added using
// the `AddReceipt` method.
//
// If ErrMaxViewDeltaExceeded is returned, the provided result was not added to the forest, and
// the caller must use `ResetLowestRejectedView` to perform the backfill process once the forest
// has available capacity.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddSealedResult(result *flow.ExecutionResult) error {
	resultID := result.ID()

	container, err := rf.getOrCreateContainer(result, BlockStatusSealed)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%s): %w", resultID, err)
	}
	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return nil
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	// sealed results must be provided in ancestor first order. if updating the last sealed view fails,
	// this indicates that a sealed result was provided out of order, which is strictly disallowed.
	if !rf.updateLastSealed(container) {
		return fmt.Errorf("failed to update last sealed view for result (%s): parent result not found in forest or is not sealed", resultID)
	}

	return nil
}

// AddReceipt adds the given execution result to the forest. Furthermore, we track which Execution Node issued
// receipts committing to this result. This method is idempotent.
//
// If ErrMaxViewDeltaExceeded is returned, the result for the provided receipt was not added to the
// forest, and the caller must use `ResetLowestRejectedView` to perform the backfill process once
// the forest has available capacity.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddReceipt(receipt *flow.ExecutionReceipt, blockStatus BlockStatus) (bool, error) {
	resultID := receipt.ExecutionResult.ID()

	container, err := rf.getOrCreateContainer(&receipt.ExecutionResult, blockStatus)
	if err != nil {
		return false, fmt.Errorf("failed to get container for result (%s): %w", resultID, err)
	}
	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return false, nil
	}

	added, err := container.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its container: %w", err)
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	container.SetBlockStatus(blockStatus)

	if blockStatus == BlockStatusSealed {
		// it is OK to receive out of order receipt notifications since they may be provided based
		// on network events.
		_ = rf.updateLastSealed(container)
	}

	return added > 0, nil
}

// updateLastSealed updates the last sealed view and abandons all orphaned sibling forks if and only
// if the result's parent exists in the forest and is already sealed. Otherwise, false is returned
// and the operation is a no-op. This function is idempotent.
//
// This guarantees that there is always a direct ancestoral connection from the latest persisted
// sealed result to the latest sealed result.
//
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) updateLastSealed(container *ExecutionResultContainer) bool {
	parentID, _ := container.Parent()
	parent, found := rf.getContainer(parentID)
	if !found || parent.BlockStatus() != BlockStatusSealed {
		return false // invariant violation, do not update
	}

	if !rf.lastSealedView.Set(container.BlockView()) {
		return true // duplicate event
	}

	// abandon all orphaned sibling forks
	for sibling := range rf.iterateChildren(parentID) {
		if sibling.ResultID() != container.ResultID() {
			rf.abandonFork(sibling)
		}
	}

	return true
}

// getOrCreateContainer retrieves or creates the container for the given result within the forest.
// It is optimized for the case of many concurrent reads compared to relatively few writes, and
// rarely repeated calls.
// getOrCreateContainer is idempotent and atomic: it always returns the first container created
// for the given result.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) getOrCreateContainer(result *flow.ExecutionResult, blockStatus BlockStatus) (*ExecutionResultContainer, error) {
	// First, try to get existing container - this will acquire read-lock only
	resultID := result.ID()
	container, found := rf.GetContainer(resultID)
	if found {
		return container, nil
	}

	// At this point, we know that the result is not *yet* in the forest. We are optimistically creating the
	// container now, while still permitting concurrent access to the forest for performance reasons.
	// Note: we are not holding the lock atm! This is beneficial, because querying for the block header
	// might fall back on a database read. Furthermore, heap allocations are more expensive operations.
	// (see https: //go101.org/optimizations/0.3-memory-allocations.html). In comparison, lock
	// allocations are assumed to be cheaper in a non-congested environment.
	executedBlock, err := rf.headers.ByBlockID(result.BlockID)
	if err != nil {
		// this is an exception since only results for certified blocks should be added to the forest
		return nil, fmt.Errorf("failed to get block header for result (%s): %w", resultID, err)
	}

	pipeline := rf.pipelineFactory.NewPipeline(result, blockStatus == BlockStatusSealed)
	newContainer, err := NewExecutionResultContainer(result, executedBlock, blockStatus, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to create container for result (%s): %w", resultID, err)
	}

	// Now, we have the container ready to be inserted. Repeat check for the container's existence and
	// proceed only with the optimisitically-constructed container if still nothing is in the forest.
	// This is implemented as an atomic operation, i.e. holding the lock.
	// In the rare case that a container for the requested result was already added by another thread concurrently,
	// the levelled forest will automatically discard it as a duplicate. Specifically, the LevelledForest considers
	// two Vertices as equal if they have the same ID (here `resultID`), Level (here `executedBlock.View`), and
	// Parent (here `result.PreviousResultID` & `executedBlock.ParentView`). In our case, all of those are guaranteed to
	// be identical for all `ExecutionResultContainer` instances that we create for the same result.
	// Though, it should be noted that different instances of the `ExecutionResultContainer` may differ in the content of
	// their `Pipeline` and `Receipts` fields (domain-specific aspects not covered by the LevelledForest).
	// Hence, we have the IMPORTANT requirement: in case a container already exists, it takes precident over our
	// optimistically constructed instance.
	rf.mu.Lock()
	defer rf.mu.Unlock()

	container, found = rf.getContainer(resultID) // early exit in case some other thread already added the container
	if found {
		return container, nil
	}
	container = newContainer

	// check invariant: the result's block view must be greater than 𝓹.View
	if executedBlock.View < rf.forest.LowestLevel {
		return nil, nil
	}

	// check invariant: the result's block view must be less than or equal to the view horizon 𝓱
	if executedBlock.View > rf.forest.LowestLevel+rf.maxViewDelta {
		rf.rejectedResults = true
		return nil, ErrMaxViewDeltaExceeded
	}

	// verify and add to forest
	err = rf.forest.VerifyVertex(container)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt's container: %w", err)
	}
	rf.forest.AddVertex(container)

	// abandon all descendants if the container is already known to be abandoned.
	if rf.isAbandonedFork(container) {
		rf.abandonFork(container)
	}

	return container, nil
}

// HasReceipt checks if a receipt exists in the forest.
func (rf *ResultsForest) HasReceipt(receipt *flow.ExecutionReceipt) bool {
	resultID := receipt.ExecutionResult.ID()
	receiptID := receipt.ID()

	rf.mu.RLock()
	defer rf.mu.RUnlock()

	container, found := rf.getContainer(resultID)
	if !found {
		return false
	}
	return container.Has(receiptID)
}

// Size returns the number of results stored in the forest.
func (rf *ResultsForest) Size() uint {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return uint(rf.forest.GetSize())
}

// LowestView returns the lowest view where results are still stored in the mempool.
func (rf *ResultsForest) LowestView() uint64 {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.forest.LowestLevel
}

// GetContainer retrieves the ExecutionResultContainer for the given result ID.
func (rf *ResultsForest) GetContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.getContainer(resultID)
}

// getContainer retrieves the ExecutionResultContainer for the given result ID without locking.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) getContainer(resultID flow.Identifier) (*ExecutionResultContainer, bool) {
	container, ok := rf.forest.GetVertex(resultID)
	if !ok {
		return nil, false
	}
	return container.(*ExecutionResultContainer), true
}

// IterateChildren returns an iter.Seq that iterates over all children of the given result ID
func (rf *ResultsForest) IterateChildren(resultID flow.Identifier) iter.Seq[*ExecutionResultContainer] {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.iterateChildren(resultID)
}

// iterateChildren returns an iter.Seq that iterates over all children of the given result ID
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) iterateChildren(resultID flow.Identifier) iter.Seq[*ExecutionResultContainer] {
	return func(yield func(*ExecutionResultContainer) bool) {
		siblings := rf.forest.GetChildren(resultID)
		for siblings.HasNext() {
			sibling := siblings.NextVertex().(*ExecutionResultContainer)
			if !yield(sibling) {
				return
			}
		}
	}
}

// IterateView returns an iter.Seq that iterates over all containers who's executed block has the given view
func (rf *ResultsForest) IterateView(view uint64) iter.Seq[*ExecutionResultContainer] {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.iterateView(view)
}

// iterateView returns an iter.Seq that iterates over all containers who's executed block has the given view
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) iterateView(view uint64) iter.Seq[*ExecutionResultContainer] {
	return func(yield func(*ExecutionResultContainer) bool) {
		containers := rf.forest.GetVerticesAtLevel(view)
		for containers.HasNext() {
			container := containers.NextVertex().(*ExecutionResultContainer)
			if !yield(container) {
				return
			}
		}
	}
}

// OnBlockFinalized notifies the ResultsForest that a block has been finalized.
// The forest updates the state of all results affected by the finalized block and its seals.
func (rf *ResultsForest) OnBlockFinalized(finalized *flow.Block) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// ignore duplicate events
	if !rf.lastFinalizedView.Set(finalized.View) {
		return nil
	}

	// abandon all forks who's executed block conflicts with the new finalized block. This is an
	// optimization since all results for these conflicting results will eventually be abandoned
	// when sealing catches up.
	//
	// Note: it is possible that there are no results in the forest for either/both the finalized
	// block or its parent. That is OK since they will eventually be abandoned.
	//
	// Steps:
	// 1. Iterate all results for the finalized block's parent
	// 2. For each of the results' children, mark the result as finalized if its executed block
	//    matches the finalized block, otherwise, abandon the fork.
	for container := range rf.iterateView(finalized.ParentView) {
		for child := range rf.iterateChildren(container.ResultID()) {
			if child.BlockView() == finalized.View {
				container.SetBlockStatus(BlockStatusFinalized)
			} else {
				rf.abandonFork(child)
			}
		}
	}

	// the following logic guarantees the sealed result notifications are processed in ancestor
	// first order, which is required by the forest to ensure the last sealed view invariant holds.
	// However, the protocol does not guarantee an order for seals within a block, only that all
	// seals within the block form a continuous chain extending from the seals in its ancestors.
	// Therefore, we need to explicitly sort the seals by view before processing.

	// 1. collect the containers pertaining to the seals
	sealedContainers := make([]*ExecutionResultContainer, 0, len(finalized.Payload.Seals))
	for _, seal := range finalized.Payload.Seals {
		container, found := rf.getContainer(seal.ResultID)
		if found {
			sealedContainers = append(sealedContainers, container)
		}
	}

	// 2. sort the containers by view in ascending order
	sort.Slice(sealedContainers, func(i, j int) bool {
		return sealedContainers[i].BlockView() < sealedContainers[j].BlockView()
	})

	// 3. process the seals -> mark the containers as sealed, update the last sealed view, and
	// abandon all conflicting forks.
	for _, container := range sealedContainers {
		container.SetBlockStatus(BlockStatusSealed)
		if !rf.updateLastSealed(container) {
			if rf.rejectedResults {
				// if results have been rejected, then it's expected some sealed results are missing.
				// abort after the first failure since all subsequent results will also fail.
				break
			}
			// otherwise, this is an invariant violation and the forest is in an inconsistent state.
			return fmt.Errorf("failed to update last sealed view for result (%s): parent result not found in forest or is not sealed", container.ResultID())
		}
	}
	return nil
}

// abandonFork recursively abandons a container and all its descendants.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) abandonFork(container *ExecutionResultContainer) {
	container.Pipeline().Abandon()
	for child := range rf.iterateChildren(container.ResultID()) {
		rf.abandonFork(child)
	}
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
func (rf *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState optimistic_sync.State) {
	// abandoned status is propagated to all descendants synchronously, so no need to traverse again here.
	if newState == optimistic_sync.StateAbandoned {
		return
	}

	// send state update to all children.
	for child := range rf.IterateChildren(resultID) {
		child.Pipeline().OnParentStateUpdated(newState)
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
//
// WARNING: we are assuming a strict ordering of calls to `processCompleted` from completed pipelines.
// This is because we require a strict ancestor first order of `StateComplete`in order of sealing to
// ensure that the forest is in a consistent state.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) processCompleted(resultID flow.Identifier) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// first, ensure that the result ID is in the forest, otherwise the forest is in an inconsistent state
	container, found := rf.getContainer(resultID)
	if !found {
		return fmt.Errorf("result %s not found in forest", resultID)
	}

	// next, ensure that this result matches the latest persisted sealed result, otherwise
	// the forest is in an inconsistent state since persisting must be done sequentially
	// Note: In practise this means that we are expecting a strict sequentiality of events `OnStateUpdated` when
	// the next pipeline reaches `optimistic_sync.StateComplete`.
	latestResultID, _ := rf.latestPersistedSealedResult.Latest()
	if container.ResultID() != latestResultID {
		return fmt.Errorf("completed result %s does not match latest persisted sealed result %s",
			container.ResultID(), latestResultID)
	}

	// finally, prune the forest up to the latest persisted result's block view
	latestPersistedView := container.BlockView()
	err := rf.forest.PruneUpToLevel(latestPersistedView)
	if err != nil {
		return fmt.Errorf("failed to prune results forest (view: %d): %w", latestPersistedView, err)
	}

	return nil
}

// isAbandonedFork checks if a container's result descends from an abandoned result.
// This requires that there is a direct path from the container's result to an abandoned result
// within the forest, otherwise the function cannot determine if the fork is abandoned and returns false.
//
// A container is known to be on an abandoned fork if:
//  1. its parent is abandoned
//  2. one of its siblings is sealed
//  3. its block conflicts with a finalized block
//
// In the case of 3, it is difficult to determine that the block conflicts without traversal.
// However, one of the result's siblings will eventually be sealed, at which point this result
// will be abandoned. Skipping the check here simplifies the logic, and forgoes the optimization
// of abandoning the fork early.
//
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) isAbandonedFork(container *ExecutionResultContainer) bool {
	// optimization: if the container is sealed, it can't be in an abandoned fork
	if container.BlockStatus() == BlockStatusSealed {
		return false
	}

	// if the parent is not found, that means either the parent has not been added to the forest yet,
	// or it has already been pruned. either way, we can't confirm if the fork is abandoned.
	parentID, _ := container.Parent()
	parent, found := rf.getContainer(parentID)
	if !found {
		return false
	}

	// 1. the parent is abandoned
	if parent.Pipeline().GetState() == optimistic_sync.StateAbandoned {
		return true
	}

	isAbandoned := false
	for sibling := range rf.iterateChildren(parentID) {
		if sibling.ResultID() == container.ResultID() {
			continue // skip self
		}

		// 2. a sibling is sealed
		if sibling.BlockStatus() == BlockStatusSealed {
			isAbandoned = true
			break
		}
	}

	return isAbandoned
}
