package ingestion2

import (
	"fmt"
	"sort"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
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
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
type ResultsForest struct {
	log                         zerolog.Logger
	forest                      forest.LevelledForest
	manager                     *ForestManager
	headers                     storage.Headers
	maxViewDelta                uint64
	lowestRejectedView          uint64
	lastSealedResultID          flow.Identifier
	lastSealedView              uint64
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader
	pipelineFactory             optimistic_sync.PipelineFactory

	mu sync.RWMutex
}

// NewResultsForest creates a new instance of ResultsForest.
// No errors are expected during normal operations.
func NewResultsForest(
	log zerolog.Logger,
	headers storage.Headers,
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader,
	manager *ForestManager,
	maxViewDelta uint64,
) (*ResultsForest, error) {
	resultID, sealedHeight := latestPersistedSealedResult.Latest()
	sealedHeader, err := headers.ByHeight(sealedHeight) // by protocol convention, this should always exist (initialized during bootstrapping)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header for latest persisted sealed result (height: %d): %w", sealedHeight, err)
	}

	rf := &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest.NewLevelledForest(sealedHeader.View),
		manager:                     manager,
		headers:                     headers,
		maxViewDelta:                maxViewDelta,
		lastSealedResultID:          resultID,
		latestPersistedSealedResult: latestPersistedSealedResult,
	}
	return rf, nil
}

// ResetLowestRejectedView resets the lowest rejected view and returns the previous value.
// If no results have been rejected since the last call, false is returned.
//
// Lowest rejected view tracks the lowest view of a result that was rejected by the forest because
// the forest's view window was exceeded. This is used by the higher-level business logic to identify
// the view from which to start reproviding results. By reproviding sealed results starting from this
// view, system can guarantee that all sealed results are eventually provided to the ResultsForest.
func (rf *ResultsForest) ResetLowestRejectedView() (uint64, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	lowestRejectedView := rf.lowestRejectedView
	rf.lowestRejectedView = 0
	return lowestRejectedView, lowestRejectedView > 0
}

// AddSealedResult adds a SEALED Execution Result to the Result Forest (without any receipts),
// in case the result is not already stored in the tree. If the result is already stored,
// its status is set to sealed.
// Execution results with a finalized seal are committed to the chain and considered final
// irrespective of which Execution Nodes [ENs] produced them. Therefore, it is fine to not track
// which ENs produced the result (but we also don't preclude receipt from being added for the
// result later).
//
// In contrast, for results without a finalized seal, it depends on the clients how much trust they
// are willing to place in the results correctness - potentially depending on how many ENs and/or
// which ENs specifically produced the result. Therefore, unsealed results must be added using
// the `AddReceipt` method.
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddSealedResult(result *flow.ExecutionResult) error {
	container, err := rf.getOrCreateContainer(result)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%s): %w", result.ID(), err)
	}
	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return nil
	}

	// This call might be the first time, where the ResultsForest learns that the result is sealed.
	container.Pipeline().SetSealed()
	return nil
}

// AddReceipt adds the given execution result to the forest. Furthermore, we track which Execution Node issued
// receipts committing to this result. This method is idempotent
//
// Expected errors during normal operations:
//   - ErrMaxViewDeltaExceeded: if the result's block view is more than maxViewDelta views ahead of the last sealed view
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (rf *ResultsForest) AddReceipt(receipt *flow.ExecutionReceipt) (bool, error) {
	container, err := rf.getOrCreateContainer(&receipt.ExecutionResult)
	if err != nil {
		return false, fmt.Errorf("failed to get container for result (%s): %w", receipt.ExecutionResult.ID(), err)
	}
	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return false, nil
	}

	added, err := container.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its container: %w", err)
	}

	rf.manager.OnReceiptAdded(container)

	return added > 0, nil
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
func (rf *ResultsForest) getOrCreateContainer(result *flow.ExecutionResult) (*ExecutionResultContainer, error) {
	// First, try to get existing container - this will acquire read-lock only
	resultID := result.ID()
	container, found := rf.GetContainer(resultID)
	if found {
		return container, nil
	}

	// At this point, we know that the result is not *yet* in the forest. We are optimistically
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

	pipeline := rf.pipelineFactory.NewPipeline(result)
	container, err = NewExecutionResultContainer(result, executedBlock, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to create container for result (%s): %w", resultID, err)
	}

	// Now, we have the container ready to be inserted. Repeat check for the container's existence and
	// proceed only with the optimisitically-constructed container if still nothing is in the forest.
	// This is implemented as an atomic operation, i.e. holding the lock.
	// In the rare case that a container for the requested result was already added by another thread concurrenlty,
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

	// drop receipts for block views lower than the lowest view.
	if executedBlock.View < rf.forest.LowestLevel {
		return nil, nil
	}

	// make sure the result's block view is within the accepted range
	if executedBlock.View > rf.forest.LowestLevel+rf.maxViewDelta {
		if rf.lowestRejectedView == 0 || executedBlock.View < rf.lowestRejectedView {
			rf.lowestRejectedView = executedBlock.View
		}
		return nil, ErrMaxViewDeltaExceeded
	}

	// Verify and add to forest
	err = rf.forest.VerifyVertex(container)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt's container: %w", err)
	}
	rf.forest.AddVertex(container)

	// mark the container as abandoned if it does not descend from the latest sealed result.
	//
	// consider the following case:
	// X is the result that was just added
	// A was previously sealed, and B was sealed before X was added
	//
	//   ↙ X
	// A ← B ← C
	//
	// in this case, we know that X conflicts with B and will never be sealed. We should abandon X
	// immediately.
	//
	// consider another case:
	// Y is the result that was just added
	// X is its parent, but does not exist in the forest yet
	//
	//   ↙ [X] ← Y
	// A ←  B  ← C
	//
	// in this case, we do not know if Y will eventually be sealed since we don't know which result
	// X will descend from. We must wait until we eventually receive X to determine if X and Y should
	// be abandoned.
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
// CAUTION:
//   - The returned ExecutionResultContainer is not concurrency safe!
//   - Code outside of the ResultsForest should NEVER MODIFY the returned ExecutionResultContainer.
//   - Accessing the returned ExecutionResultContainer in a thread other than the thread servicing the ResultsForest,
//     may lead to panics.
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

// IterateChildren iterates over all children of the given result ID and calls the provided function on each child.
// CAUTION: this will aquire a read lock on the ResultsForest, so it is safe to call concurrently.
// Callback function should return false to stop iteration
func (rf *ResultsForest) IterateChildren(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	rf.iterateChildren(resultID, fn)
}

// iterateChildren iterates over all children of the given result ID and calls the provided function on each child.
// Callback function should return false to stop iteration
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) iterateChildren(resultID flow.Identifier, fn func(*ExecutionResultContainer) bool) {
	siblings := rf.forest.GetChildren(resultID)
	for siblings.HasNext() {
		sibling := siblings.NextVertex().(*ExecutionResultContainer)
		if !fn(sibling) {
			return
		}
	}
}

// OnResultSealed marks the execution result as sealed and updates the state of related pipelines.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) OnResultSealed(resultID flow.Identifier) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Get the container for the newly sealed result
	sealedContainer, found := rf.getContainer(resultID)
	if !found {
		// the sealed result may not be loaded yet. we can ignore this notification for now and
		// handle it during a future call.
		return nil
	}

	// collect all unsealed containers in the path from the sealed result to the last sealed result.
	// this ensures:
	// 1. the newly sealed result descends from the last sealed result (state is consistent)
	// 2. any sealing notifications that were missed due to undiscovered results are handled
	// 3. sealing notifications are processed in sealing order
	unsealedContainers, err := rf.findUnsealedAncestors(sealedContainer)
	if err != nil {
		return err
	}

	// if this notification is for an already sealed result, we can ignore it.
	if len(unsealedContainers) == 0 {
		return nil
	}

	for _, container := range unsealedContainers {
		rf.markResultSealed(container)
	}

	rf.lastSealedResultID = resultID
	rf.lastSealedView = sealedContainer.BlockView()

	return nil
}

// findUnsealedAncestors returns all unsealed containers between the last sealed container (excluded)
// and the provided `head` of an execution fork. The returned containers are ordered by ascending views
// of the executed blocks.
// CAUTION: not concurrency safe!
//
// findUnsealedAncestors expects that `head` is a descendant of the last sealed result and that all
// intermediate results are also in the forest. Otherwise, an exception is returned.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) findUnsealedAncestors(head *ExecutionResultContainer) ([]*ExecutionResultContainer, error) {
	unsealedContainers := make([]*ExecutionResultContainer, 0)

	if rf.lastSealedView >= head.BlockView() {
		return unsealedContainers, nil
	}
	unsealedContainers = append(unsealedContainers, head)

	for {
		parentID, _ := head.Parent()
		if parentID == rf.lastSealedResultID {
			break
		}

		parent, found := rf.getContainer(parentID)
		if !found {
			return nil, fmt.Errorf("ancestor result %s of %s not found in forest", parentID, head.ResultID())
		}

		unsealedContainers = append(unsealedContainers, parent)
		head = parent
	}

	// Sort containers by view in ascending order
	sort.Slice(unsealedContainers, func(i, j int) bool {
		return unsealedContainers[i].BlockView() < unsealedContainers[j].BlockView()
	})

	return unsealedContainers, nil
}

// markResultSealed marks a result as sealed and updates its siblings' pipelines to abandoned.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) markResultSealed(container *ExecutionResultContainer) {
	container.Pipeline().SetSealed()

	// abandon all conflicting forks
	parentID, _ := container.Parent()
	rf.iterateChildren(parentID, func(sibling *ExecutionResultContainer) bool {
		if sibling.ResultID() != container.ResultID() {
			rf.abandonFork(sibling)
		}
		return true
	})
}

// OnBlockStatusUpdated signals that the block status has been updated.
// It finds all vertices for results of blocks that conflict with the finalized block and abort them.
func (rf *ResultsForest) OnBlockStatusUpdated(finalized *flow.Header, sealed *flow.Header, parentBlockResultIDs []flow.Identifier) error {
	finalizedBlockID := finalized.ID()
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// 1. Get all ExecutionResults for the finalized block's parent (done by caller)
	// 2. For each of these results, get all child vertices
	// 3. For each child vertex, cancel if it does not reference the finalized block

	for _, parentResultID := range parentBlockResultIDs {
		rf.iterateChildren(parentResultID, func(child *ExecutionResultContainer) bool {
			if child.Result().BlockID != finalizedBlockID {
				rf.abandonFork(child)
			}
			return true
		})
	}

	rf.manager.OnBlockStatusUpdated(finalized, sealed)
	return nil
}

// abandonFork recursively abandons a container and all its descendants.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) abandonFork(container *ExecutionResultContainer) {
	container.Pipeline().Abandon()
	rf.iterateChildren(container.ResultID(), func(child *ExecutionResultContainer) bool {
		rf.abandonFork(child)
		return true
	})
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
//
// WARNING: we are assuming a strict ordering of events of the type `OnStateUpdated` from different pipelines
// across the forest. This is because `processCompleted` requires a strict ancestor first order of `StateComplete`
// in order of sealing.
func (rf *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState optimistic_sync.State) {
	// abandoned status is propagated to all descendants synchronously, so no need to traverse again here.
	if newState == optimistic_sync.StateAbandoned {
		return
	}

	// send state update to all children.
	rf.IterateChildren(resultID, func(child *ExecutionResultContainer) bool {
		child.Pipeline().OnParentStateUpdated(newState)
		return true
	})

	// process completed pipelines
	if newState == optimistic_sync.StateComplete {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if err := rf.processCompleted(resultID); err != nil {
			// TODO: handle with a irrecoverable error
			rf.log.Fatal().Err(err).Msg("irrecoverable exception: failed to process completed pipeline")
		}
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
// CAUTION: not concurrency safe! Caller must hold a lock.
//
// No errors are expected during normal operation.
func (rf *ResultsForest) processCompleted(resultID flow.Identifier) error {
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
// within the forest, a sibling of a sealed result.
// If any result is missing, the fork is not known to be abandoned and the function returns false.
//
// Since it is possible that the result's sibling is sealed (thus its parent is also sealed), we need
// to traverse to the latest sealed result to determine if the fork is abandoned.
//
// CAUTION: not concurrency safe! Caller must hold a lock.
func (rf *ResultsForest) isAbandonedFork(container *ExecutionResultContainer) bool {
	parentID, parentView := container.Parent()

	// the parent is the latest sealed result, so the fork is most likely not abandoned.
	// the only exception is if the result's block conflicts with a finalized block.
	// TODO: how to handle conflicting block case?
	if parentID == rf.lastSealedResultID {
		return false
	}

	// sealed views are strictly increasing, so if we find a parent view that is lower than the
	// last sealed view, and we have not yet found the latest sealed result, then we are guaranteed
	// to never find it. This catches the case where the result's sibling is sealed.
	if parentView < rf.lastSealedView {
		return true
	}

	// if the parent is not found, that means either the parent has not been added to the forest yet,
	// or it has already been pruned. either way, we can't confirm if the fork is abandoned.
	parent, found := rf.getContainer(parentID)
	if !found {
		return false
	}

	// if the parent is abandoned, then the container does not descend from the latest sealed
	// result, and we can guarantee that the container will never be started.
	return parent.Pipeline().GetState() == optimistic_sync.StateAbandoned
}
