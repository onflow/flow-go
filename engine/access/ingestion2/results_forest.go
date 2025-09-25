package ingestion2

import (
	"errors"
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

	// ErrPrunedView is returned when attempting to add a result whose view is below the lowest view.
	ErrPrunedView = fmt.Errorf("result's block is below the forest's view cutoff")
)

// <component_spec>
//
// ResultsForest is a mempool holding execution results (and receipts), which is aware of the tree
// structure formed by the results. The mempool provides functional primitives for deciding if and when
// data for an execution result should be downloaded, processed, persisted, and/or abandoned.
//
// # USAGE PATTERN
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
//     the receipts, and processing status of each result. While it is concurrency safe, it utilizes the calling
//     threads to execute its internal logic.
//
// # NOMENCLATURE
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
// # FORMAL REQUIREMENTS
// Conceptually, the ResultsForest maintains the following three quantities:
//  1. 𝓹 tracks the 𝙡𝙤𝙬𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝘄𝗶𝘁𝗵𝗶𝗻 the ResultsForest. Specifically
//     (i) the ResultsForest knows 𝓹 to be sealed and
//     (ii) no results with a lower view exist in the forest.
//  2. 𝓼 is a local notion of the 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝘄𝗶𝘁𝗵𝗶𝗻 𝘁𝗵𝗲 𝗳𝗼𝗿𝗲𝘀𝘁. Specifically, we require all
//     of the following attributes to hold:
//     (i) The ResultsForest knows all acestor results of 𝓼 up to and including 𝓹 (by virtue of being sealed,
//     we know that 𝓹 must be an ancestor of 𝓼). In other words, recursing the execution fork from 𝓼 backwards
//     following the `PreviousResultID`, we eventually will reach 𝓹.
//     (ii) The ResultsForest knows 𝓼 and all its acestor results up to and including 𝓹 to be sealed.
//     (iii) No other result 𝒓 resists in the forest that satisfies (i) and (ii) but has a higher view than 𝓼.
//     Note that this definition purposefully excludes results that have been sealed by the consensus nodes,
//     but which the ResultsForest hasn't ingested yet or where some ancestor results are not yet available.
//  3. 𝓱 is the ResultsForest's 𝙫𝙞𝙚𝙬 𝙝𝙤𝙧𝙞𝙯𝙤𝙣. The result forest must store any results with views in the closed
//     interval [𝓹.Level, 𝓱]. The implementation sets 𝓱 = 𝓹.Level + maxViewDelta. Results with views
//     outside *may* be rejected. The following is a degree of freedom for our design:
//     ○ Theoretically there is no bound on how many consensus views can pass _without_ new blocks being produced.
//     Nevertheless, for practical considerations, we have already introduced the axiom that within every window of
//     `FinalizationSafetyThreshold` views (for details see [protocol.GetFinalizationSafetyThreshold] ), at least one
//     block must be finalized. `FinalizationSafetyThreshold` is chosen such that a violation of this axiom has vanshing
//     probability. The axiom implies that two blocks with ancestral degree 1 (i.e. parent and child) cannot be more than
//     `FinalizationSafetyThreshold` views apart. Hence, as long as 𝓱 - 𝓹.Level ≥ `FinalizationSafetyThreshold`, the
//     child of 𝓹 always falls into the [𝓹.Level, 𝓱].
//     ○ In case we choose `maxViewDelta` := 𝓱 - 𝓹.Level < `FinalizationSafetyThreshold`, we cannot guarantee that the
//     child of 𝓹 will fall into the view range [𝓹.Level, 𝓱]. Therefore, we introduce an additional requirement that the
//     ResultsForest must always store the immediate children of 𝓹, even if they have views greater than 𝓱. We emphasize
//     that allowing 𝓱 to be closer to 𝓹.Level limits the Access Nodes' memory consumption, which is favourable for some
//     node operators. Therefore, we adopt this approach.
//     ➩ In summary, the ResultsForest must always include the children (ancestry degree 1) of 𝓹. For a result 𝒓 with
//     larger ancestry degree or (partially) unknown ancestry, we include 𝒓 if and only if 𝒓.Level is in the closed
//     interval [𝓹.Level, 𝓱].
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
//   - 𝓹.Level, 𝓼.Level, 𝓱, monotonically increase during the runtime of the ResultsForest
//   - [optional, simplifying convention] there exists no result 𝒓 in the forest with 𝒓.Level < 𝓹.Level
//
// Any honest protocol execution should satisfy the invariant. Hence, the invariant being violated is a
// symptom of a severe bug in the protocol implementation or a corrupted internal state. Either way, safe
// continuation is not possible and the node should restart.
// Lastly, we require that the ResultsForest eventually progresses up to 𝓼, without relying on any input
// events from the consensus follower (`AddReceipt`, `OnBlockFinalized`, `AddSealed`). This requirement can
// be satisifed by the ResultsForest on its own, since it already has the chain of results between 𝓹 (latest
// sealed result whose data was successfully ingested) and 𝓼 (newest sealed result, up to which data can be
// ingested). Note that we are assuming liveness of the encapsulated [optimistic_sync.Pipeline] instances
// for sealed blocks.
//
// # CONTRACT WITH DATA SOURCE
// The result forest requires that finalized block and sealed result notifications are both delivered
// in ancestor first order. This is relaxed for unsealed results which may be delivered in any order.
//
// With the following additional 𝒄𝒐𝒏𝒕𝒓𝒂𝒄𝒕, the invariant is sufficient to guarantee safety and liveness
// of the ResultsForest (proof sketched below):
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
// # RECOVERY FROM CRASHES
//   - The latest sealed result whose execution data was successfully ingested and persisted in the database
//     must be provided as value for 𝓹 during construction of the ResultsForest.
//   - All sealed results descending from 𝓹 that the consensus follower knows about must be added via the
//     `AddSealedResult` in ancestor first order.
//   - Any execution receipts incorporated in blocks descending from the latest finalized block known to the
//     consensus follower must be added via `AddReceipt` in ancestor first order.
//
// Recovering the `ResultsForest`s internal state must be completed before the consensus follower starts
// processing new blocks. Thereby, subsequent notifications from the consensus follower are guaranteed to
// satisfy the ancestor first order for all notifications that pertain to non-orphaned blocks and results.
//
// # IMPORTANT DESIGN ASPECTS:
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
//     This is another case, where the ResultsForest's local notion lags behind the protocol's global view, which
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
//
// # SAFETY AND LIVENESS
// Safety here means that only results marked as sealed by the protocol will be considered sealed by the ResultsForest. This
// is relatively straightforward to verify directly from the implementation. In the following formal agument, we focus on
// liveness, which means that every result sealed by the protocol must eventually be considerd _processed_ by the ResultsForest.
//
// As we require (specified above) that the data source delivers result in ancestor-first order, there are only two scenarios
// where ancestry can be unknown from the ResultsForest's perspective:
//
//	 (a) The forest rejected a result in the ancestry.
//	 (b) The parent result is below the pruning threshold. In this case, the result itself is orphaned if and only if
//		 the result is different than 𝓹.
//
// Scenario I: The following argument proves that the ResultsForest will be live as long as it does not reject any results.
//   - New results continue being delivered in an ancestor-first order with all ancestors being known (by induction, starting
//     with the forest being properly repopulated at startup).
//   - According its specification, the indexing process is guaranteed to be live up to 𝓼.
//   - As long as the forest does not reject any results, the forest's 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼
//     will continue to grow, because the forest is being fed with results in ancestor-first order.
//
// Scenario II: now we sketch the proof showing that the ResultsForest is also live after it rejected some result.
//   - Per contract, if the forest rejected results, the backfill process will eventually kick in and deliver sealed results
//     starting from 𝓼.Level up to the latest sealed result known to the consensus follower.
//   - In case the backfill process finished without driving the forest into rejecting results again:
//     Consider the next "sealing" notification from the consensus follower (`OnFinalizedBlock` notification, where the
//     finalized block contains a seal). Since the backfill process only stopped at the latest sealed result known to the consensus
//     follower, the new `OnFinalizedBlock` notification just received must seal the immediate next child. Hence, the
//     forest's 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼 increases. Furthermore, the result forest's mode of operation is back in Scenario I, which is live
//     (as long as no further rejection of inputs occurs).
//   - In case the backfill process drives the forest into rejecting results again:
//     Note that either the backfill process adds a new result to the forest that is a child of 𝓹 or such child already exists
//     within the forest. The only case where such child does not exist in the forest is if 𝓹 = 𝓼. As specified in the requirements
//     section, the forest will always accept and store 𝓹's child. Therefore, the backfill process will either add a sealed child
//     of 𝓼 or such child will be added and sealed through the notifications from the consensus follower. In either case, the
//     forest's 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼 increases and the forest will therefore make progress.
//
// In all cases, the ResultsForest makes progress and the forest's notion 𝙡𝙖𝙩𝙚𝙨𝙩 𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓼 also keeps progressing. Q.E.D.
//
// # RATIONALE ON ANCESTOR-FIRST ORDER
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
//   - Furthermore, consensus makes the following garantees for the blocks it produces, which carries
//     over to the consensus follower observing the blocks:
//     ○ In block B, the proposer may only include results that pertain to ancestors of B.
//     ○ Results are included into a fork in an ancestor-first ordering. Meaning, the proposer of
//     block B may include a result R only if the result referenced by R.PreviousResultID was
//     included in B or its ancestors.
//
// # PRUNING
// The ResultsForest mempool supports pruning by view:
// only results descending from the latest sealed and finalized result are relevant.
// By convention, the ResultsForest always contains the latest processed sealed result. Thereby, the
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
//     (a) processing of execution forks that conflict with known sealed results are abandoned
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
//   - The ResultsForest tracks the finalization status of blocks within the forest. This is used
//     by higher-level business logic (e.g. ForestManager) to determine whether a result can be
//     processed.
//   - Results with views > 𝓼.Level are not guaranteed to form a chain from 𝓼. They may be added in
//     any order. Therefore, the forest does not make any guarantees about the connectedness of
//     finalized results.
//   - Block finalization information is primarily delivered via the OnBlockFinalized event handler.
//     The forest tracks the highest finalized view for which it has received a notification. Any
//     receipt received with the result status set to ResultForFinalizedBlock, whose view is greater than
//     the forest's highest finalized view, will be not be marked finalized. Instead, the forest will
//     wait until OnBlockFinalized receives the notification for that block.
//   - This ensures that there are no results within the forest that are marked as finalized and whose
//     ancestors are not also all marked as finalized or sealed.
//
// </component_spec>
//
// All exported methods are safe for concurrent access. Internally, the mempool utilizes the
// LevelledForrest.
type ResultsForest struct {
	log             zerolog.Logger
	headers         storage.Headers
	pipelineFactory optimistic_sync.PipelineFactory

	// forest maintains the underlying graph structure of execution results. It provides the
	// following quantities from the specification:
	//  forest.LowestLevel ≡ 𝓹.Level
	forest forest.LevelledForest

	// lowestSealedResult is the *sealed* result with the _smallest_ view inside the forest.
	// Per convention, this result's data is indexed and persisted. For brevity, the
	// specification denotes this quantity as 𝓹.
	lowestSealedResult *ExecutionResultContainer

	// lastSealedView is 𝓼.Level, i.e. the view of the latest sealed result that the forest
	// can make progress on without further input from the consensus follower.
	// Deprecated: use `latestSealedResult.Level()` instead.
	lastSealedView counters.StrictMonotonicCounter

	// latestSealedResult is the sealed result with the _greatest_ view, whose ancestry is known
	// back to and including 𝓹. For brevity, the specification denotes this quantity as 𝓼.
	// As per specification, the ResultsForest should be able to progress indexing results up to
	// and including `latestSealedResult` without further input from the consensus follower.
	latestSealedResult *ExecutionResultContainer

	// lastFinalizedView is the view of the last finalized block processed by the forest.
	lastFinalizedView counters.StrictMonotonicCounter

	// latestPersistedSealedResult tracks view and ID of the latest persisted sealed result.
	// It is important to remember that information is solely from the perspective of the storage layer!
	// As forest triggers persisting of processed result data, this quantity can be *ahead* of the forest's
	// internal result 𝓹 with the smallest view. Nevertheless, conceptually `latestPersistedSealedResult`
	// must always an execution result within the fork 𝓹 ← … ← 𝓼.
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader

	// maxViewDelta specifies the number of views past its lowest view that the forest will accept.
	// The 𝙫𝙞𝙚𝙬 𝙝𝙤𝙧𝙞𝙯𝙤𝙣 𝓱 is computed as: 𝓱 = 𝓹.Level + maxViewDelta. Results for views higher than
	// view horizon are rejected, ensuring that the forest does not grow unbounded.
	maxViewDelta uint64

	// rejectedResults is a boolean value that indicates whether the ResultsForest has rejected any
	// results because their view exceeds the forest's view horizon.
	rejectedResults bool

	mu sync.RWMutex
}

// NewResultsForest creates a new instance of ResultsForest, and adds the latest persisted sealed
// result to the forest.
//
// No errors are expected during normal operations.
func NewResultsForest(
	log zerolog.Logger,
	headers storage.Headers,
	results storage.ExecutionResults,
	pipelineFactory optimistic_sync.PipelineFactory,
	latestPersistedSealedResult storage.LatestPersistedSealedResultReader,
	maxViewDelta uint64,
) (*ResultsForest, error) {
	sealedResultID, sealedHeight := latestPersistedSealedResult.Latest()

	// by protocol convention, the latest persisted sealed result and its executed block should always
	// exist. They are initialized to the root block and result during bootstrapping.
	sealedResult, err := results.ByID(sealedResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest persisted sealed result (%s): %w", sealedResultID, err)
	}

	sealedHeader, err := headers.ByHeight(sealedHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to get block header for latest persisted sealed result (height: %d): %w", sealedHeight, err)
	}

	// insert the latest persisted sealed result into the forest.
	pipeline := pipelineFactory.NewCompletedPipeline(sealedResult)
	latestSealedResultContainer, err := NewExecutionResultContainer(sealedResult, sealedHeader, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to create container for latest persisted sealed result (%s): %w", sealedResultID, err)
	}
	err = latestSealedResultContainer.SetResultStatus(ResultSealed)
	if err != nil {
		return nil, fmt.Errorf("failed to the status of result %v to sealed: %w", sealedResultID, err)
	}

	forest := forest.NewLevelledForest(sealedHeader.View)
	err = forest.VerifyVertex(latestSealedResultContainer)
	if err != nil {
		// this should never happen since this is the first container added to the forest.
		return nil, fmt.Errorf("failed to verify container: %w", err)
	}
	forest.AddVertex(latestSealedResultContainer)

	return &ResultsForest{
		log:                         log.With().Str("component", "results_forest").Logger(),
		forest:                      *forest,
		headers:                     headers,
		pipelineFactory:             pipelineFactory,
		maxViewDelta:                maxViewDelta,
		lowestSealedResult:          latestSealedResultContainer, // at initialization 𝓹 = 𝓼
		latestSealedResult:          latestSealedResultContainer,
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

	return rf.latestSealedResult.BlockView(), rejectedResults
}

// AddSealedResult adds a SEALED Execution Result to the Result Forest (without any receipts),
// in case the result is not already stored in the tree. If the result is already stored,
// its status is set to sealed.
//
// IMPORTANT: The caller MUST provide sealed results in ancestor first order to allow the forest to
// guarantee liveness. Specifically, (i) the result's ancestry must be stored in the ResultsForest going
// back to the forest's lowest sealed result _and_ (ii) the ResultsForest must know that those ancestors
// are all sealed. Otherwise, an error is returned.
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
// Expected error returns during normal operations:
//   - [ErrPrunedView]: if the result's block view is below the lowest view.
//   - [ErrMaxViewDeltaExceeded]: if the result's block view is more than maxViewDelta views ahead of the last sealed view
func (rf *ResultsForest) AddSealedResult(result *flow.ExecutionResult) error {
	resultID := result.ID()

	container, err := rf.getOrCreateContainer(result, ResultSealed)
	if err != nil {
		return fmt.Errorf("failed to get container for result (%s): %w", resultID, err)
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.updateLastSealed(container)

	return nil
}

// AddReceipt adds the given execution result to the forest. Furthermore, we track which Execution Node issued
// receipts committing to this result. This method is idempotent.
// `resultStatus` is used initialize the status of new results added to the forest. If the result is
// already known, `resultStatus` is ignored.
//
// IMPORTANT: The caller MUST provide sealed results in ancestor first order to allow the forest to
// guarantee liveness. If a receipt is added for an unknown sealed result whose parent is either
// missing or not sealed, an error is returned.
//
// If ErrMaxViewDeltaExceeded is returned, the result for the provided receipt was not added to the
// forest, and the caller must use `ResetLowestRejectedView` to perform the backfill process once
// the forest has available capacity.
//
// Expected error returns during normal operations:
//   - [ErrPrunedView]: if the result's block view is below the lowest view
//   - [ErrMaxViewDeltaExceeded]: if the result's block view is more than maxViewDelta views ahead of the last sealed view
func (rf *ResultsForest) AddReceipt(receipt *flow.ExecutionReceipt, resultStatus ResultStatus) (bool, error) {
	if !resultStatus.IsValid() {
		return false, fmt.Errorf("invalid result status: %s", resultStatus)
	}

	resultID := receipt.ExecutionResult.ID()

	container, err := rf.getOrCreateContainer(&receipt.ExecutionResult, resultStatus)
	if err != nil {
		return false, fmt.Errorf("failed to get container for result (%s): %w", resultID, err)
	}
	if container == nil {
		// noop if the result's block view is lower than the lowest view.
		return false, nil
	}

	added, err := container.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its container (%s): %w", resultID, err)
	}

	return added > 0, nil
}

// extendSealedFork extends the sealed fork 𝓹 ← … ← 𝓼, by a child result of 𝓼. The child result must already
// be stored in the forest. CAUTION: This method is NOT IDEMPOTENT.
//
// IMPORTANT: This method is the only place with the authority to
//   - update latest sealed result 𝓼, aka `latestSealedResult`
//   - set the status of the result to sealed.
//
// extendSealedFork guarantees that thr following portion of our invariant is maintained:
//   - 𝓼 always exist in the forest
//   - 𝓹 is an ancestor of 𝓼 and 𝓹.Level ≤ 𝓼.Level
//   - 𝓼.Level, 𝓱, monotonically increase during the runtime of the ResultsForest
//
// For maintaining our invariant, it is important to ensure that the fork 𝓹 ← … ← 𝓼 is correctly
// extended in conjunction with the status `ResultSealed` being assigned to only results along
// this fork. Having a single place in the code with the sole authority to verify and update the
// ResultsForest's state in this regard is important to avoid accidental invariant violations during
// future code changes and also for reviewers to easily verify that the invariant is maintained.
//
// NOT CONCURRENCY SAFE!
//
// The input must be the direct child of 𝓼, aka `latestSealedResult`, otherwise an exception is raised.
// No error returns expected during normal operations.
func (rf *ResultsForest) extendSealedFork(childResult *ExecutionResultContainer) error {
	// Enforce that purported childResult has in fact the latest sealed result as its parent. The ResultsForest's correctness working critically
	// depends on the integrity of the parent-child relationship. For our high-assurance system, we account for the possibility of bugs in other
	// components to the extent easily possible. In other words, we explicitly check the parent-child relationship here, so the ResultsForest
	// can provide intrinsic integrity guarantees, without relying on other components.
	parentID, parentView := childResult.Parent()
	if rf.latestSealedResult.ResultID() != parentID {
		return fmt.Errorf("latest sealed result is %v, while its purported child reports %v as its parent ", rf.latestSealedResult.ResultID(), parentID)
	}
	if rf.latestSealedResult.BlockView() != parentView {
		return fmt.Errorf("latest sealed result has view %d, while its purported child reports its parent view as %d", rf.latestSealedResult.BlockView(), parentView)
	}

	// The following sanity check confirms the conceptual invariant that 𝓼 must monotonically increase during the runtime
	// of the ResultsForest. A violation necessarily requires invalid inputs. Nevertheless, we check this here to ensure the
	// ResultsForest's integrity. As 𝓼.Level strictly monotonically increases, the same holds for 𝓱 = 𝓼.Level + `maxViewDelta`.
	if !(rf.latestSealedResult.BlockView() < childResult.BlockView()) {
		return fmt.Errorf("latest sealed result has view %d, while its purported child reports its parent view as %d", rf.latestSealedResult.BlockView(), parentView)
	}

	// Sanity check that the child is stored:
	if _, found := rf.getContainer(childResult.ResultID()); !found {
		return fmt.Errorf("cannot extend sealed fork by child result that is not yet stored in the forest")
	}

	// Sanity check that the child's current status is not sealed. This is expected, because `extendSealedFork` is the
	// only place in the code that sets the status to sealed, if and only if the result is appended to the sealed fork.
	if childResult.ResultStatus() == ResultSealed {
		return fmt.Errorf("sealed fork has not yet been extended by the child result, though the child is already marked as sealed")
	}

	// UPDATE INVARIANT: latest sealed result 𝓼, aka `latestSealedResult`, is updated to its child and the childs status is set to sealed.
	rf.latestSealedResult = childResult
	err := childResult.SetResultStatus(ResultSealed)
	if err != nil {
		return fmt.Errorf("failed to set status of child result to sealed: %w", err)
	}
	return nil
}

// extendsSealedChain checks if a result extends the chain from the latest persisted sealed result
// to the latest sealed result.
//
// The result is considered to extend the chain if its PreviousResultID exists in the forest and is
// sealed, and its executed block's parent view is less than or equal to the last sealed view.
//
// NOT CONCURRENCY SAFE!
func (rf *ResultsForest) extendsSealedChain(container *ExecutionResultContainer) bool {
	parentID, _ := container.Parent()
	parent, found := rf.getContainer(parentID)
	if !found || parent.ResultStatus() != ResultSealed {
		return false // invariant violation
	}

	// This check is required if we allow results to be added with sealed status with views higher
	// than the last sealed view.
	return container.BlockHeader().ParentView <= rf.lastSealedView.Value()
}

// updateLastSealed updates the last sealed view and abandons all orphaned sibling forks.
// This function is idempotent.
//
// NOT CONCURRENCY SAFE!
func (rf *ResultsForest) updateLastSealed(container *ExecutionResultContainer) {
	if !rf.lastSealedView.Set(container.BlockView()) {
		return // duplicate event
	}

	// abandon all orphaned sibling forks
	parentID, _ := container.Parent()
	for sibling := range rf.iterateChildren(parentID) {
		if sibling.ResultID() != container.ResultID() {
			rf.abandonFork(sibling)
		}
	}
}

// getOrCreateContainer retrieves or creates the container for the given result within the forest.
// When creating a new container, it is initialized with the given result status. `resultStatus` is
// ignored if the container already exists.
// This method is optimized for the case of many concurrent reads compared to relatively few writes,
// and rarely repeated calls.
// It is idempotent and atomic: it always returns the first container created for the given result.
//
// Expected error returns during normal operations:
//   - [ErrPrunedView]: if the result's block view is below the lowest view
//   - [ErrMaxViewDeltaExceeded]: if the result's block view is more than maxViewDelta views ahead
//     of the oldes stored result (lowest view) *and* the given result is not
func (rf *ResultsForest) getOrCreateContainer(result *flow.ExecutionResult, resultStatus ResultStatus) (*ExecutionResultContainer, error) {
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

	pipeline := rf.pipelineFactory.NewPipeline(result, resultStatus == ResultSealed)
	newContainer, err := NewExecutionResultContainer(result, executedBlock, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to create container for result (%s): %w", resultID, err)
	}

	// Now, we have the container ready to be inserted. Repeat check for the container's existence and
	// proceed only with the optimistically-constructed container if still nothing is in the forest.
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

	err = rf.ensureAdmissible(container)
	if err != nil {
		// TODO: as mentioned in the context of function `ensureAdmissible`, I think it would be beneficial to
		//       move this check into `ensureAdmissible` together with the insert into the forest.
		if errors.Is(err, ErrMaxViewDeltaExceeded) {
			rf.rejectedResults = true
		}
		return nil, err
	}

	// check invariant: there must always be a direct ancestoral connection from the latest persisted
	// sealed result to the latest sealed result. this is guaranteed by never accepting a sealed result
	// whose parent does not exist in the forest or is not sealed.
	if resultStatus == ResultSealed && !rf.extendsSealedChain(container) {
		return nil, fmt.Errorf("sealed result (%s) does not extend last sealed result (view: %d)", resultID, rf.lastSealedView.Value())
	}

	// verify and add to forest
	err = rf.forest.VerifyVertex(container)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt's container: %w", err)
	}
	rf.forest.AddVertex(container)
	// TODO: as mentioned in the context of function `ensureAdmissible`, I think it would be beneficial to
	//       move the LevelledForest operation (above) inside our function `ensureAdmissible`, toge
	//       However, this will require additional refactoring, to move modifications of the ResultStatus above
	//       outside the scope of `getOrCreateContainer`. I think conceptually we are holding the lock `rf.mu` here
	//       because we want to potentially add a container. We are done with that now. I think it would improve
	//       code clarity and robustness if subsequent modifications of the container would be handled by a separate
	//       method.
	//       It should be noted that, for some updates of ResultStatus, we operate solely on the container may not require
	//       holding the lock `rf.mu`. While this is of no performance concern, getting vertices from the forest is
	//       still conceptually different than operating on the vertices itself. In other words, there are similar usage
	//       patterns, but conceptually different operations.

	switch resultStatus {
	case ResultSealed:
		if err := container.SetResultStatus(resultStatus); err != nil {
			return nil, fmt.Errorf("failed to set result status to sealed: %w", err)
		}

	case ResultForFinalizedBlock:
		// only set the result status to finalized for views already processed by OnBlockFinalized.
		// Results with newer views will eventually be marked finalized by OnBlockFinalized.
		// This ensures that there are no results within the forest that are marked as finalized
		// and whose ancestors are not also all marked as finalized.
		if executedBlock.View <= rf.lastFinalizedView.Value() {
			if err := container.SetResultStatus(resultStatus); err != nil {
				return nil, fmt.Errorf("failed to set result status to finalized: %w", err)
			}
		}

	default:
		// abandon all descendants if the container is already known to be abandoned.
		if rf.isAbandonedFork(container) {
			rf.abandonFork(container)
		}
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
// NOT CONCURRENCY SAFE!
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
// NOT CONCURRENCY SAFE!
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

// IterateView returns an iter.Seq that iterates over all containers whose executed block has the given view
func (rf *ResultsForest) IterateView(view uint64) iter.Seq[*ExecutionResultContainer] {
	rf.mu.RLock()
	defer rf.mu.RUnlock()
	return rf.iterateView(view)
}

// iterateView returns an iter.Seq that iterates over all containers whose executed block has the given view
// NOT CONCURRENCY SAFE!
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

	// if the new finalized block's view is less than or equal to the last sealed view, this means
	// that the receipt notifications are ahead of the finalization notifications. In this case,
	// forest has already marked the sealed results and abandoned the conflicting forks. We can
	// safely ignore the event.
	if finalized.View <= rf.lastSealedView.Value() {
		return nil
	}

	// abandon all forks whose executed block conflicts with the new finalized block. This is an
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
	for parent := range rf.iterateView(finalized.ParentView) {
		for container := range rf.iterateChildren(parent.ResultID()) {
			if container.BlockView() == finalized.View {
				if err := container.SetResultStatus(ResultForFinalizedBlock); err != nil {
					return fmt.Errorf("failed to set result status to finalized: %w", err)
				}
				continue
			}

			// sanity check: this result's block conflicts with the latest finalized block. it must
			// not be marked as finalized or the forest is in an inconsistent state.
			if container.ResultStatus() == ResultForFinalizedBlock {
				return fmt.Errorf("result (%s) was marked finalized, but its block's view (%d) is different from the latest finalized block (%d)",
					container.ResultID(), container.BlockView(), finalized.View)
			}

			rf.abandonFork(container)
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
		// skip any missing containers for now, they will be handled in step 3.
	}

	// 2. sort the containers by view in ascending order
	sort.Slice(sealedContainers, func(i, j int) bool {
		return sealedContainers[i].BlockView() < sealedContainers[j].BlockView()
	})

	// 3. process the seals -> mark the containers as sealed, update the last sealed view, and
	// abandon all conflicting forks.
	for _, container := range sealedContainers {
		if !rf.extendsSealedChain(container) {
			if rf.rejectedResults {
				// if results have been rejected, then it's expected some sealed results are missing.
				// abort after the first failure since all subsequent results will also fail.
				break
			}
			// otherwise, this is an invariant violation and the forest is in an inconsistent state.
			return fmt.Errorf("failed to update last sealed view for result (%s): parent result not found in forest or is not sealed",
				container.ResultID())
		}
		if err := container.SetResultStatus(ResultSealed); err != nil {
			return fmt.Errorf("failed to set result status to sealed: %w", err)
		}
		rf.updateLastSealed(container)
	}
	return nil
}

// OnStateUpdated is called by pipeline state machines when their state changes, and propagates the
// state update to all children of the result.
//
// WARNING: we are assuming a strict ordering of events of the type `OnStateUpdated` from different
// pipelines across the forest. This is because `processCompleted` requires a strict ancestor first
// order of `StateComplete` in order of sealing.
func (rf *ResultsForest) OnStateUpdated(resultID flow.Identifier, newState optimistic_sync.State) {
	// abandoned status is propagated to all descendants synchronously, so no need to traverse again here.
	if newState == optimistic_sync.StateAbandoned {
		return
	}

	// send state update to all children.
	for child := range rf.IterateChildren(resultID) {
		child.Pipeline().OnParentStateUpdated(newState)
	}

	// process completed pipelines
	if newState == optimistic_sync.StateComplete {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		if err := rf.processCompleted(resultID); err != nil {
			// TODO: handle with a irrecoverable error
			rf.log.Fatal().Err(err).Msg("irrecoverable exception: failed to process completed pipeline")
			return
		}
	}
}

// processCompleted processes a completed pipeline and prunes the forest.
// NOT CONCURRENCY SAFE!
//
// No error returns are expected during normal operation.
func (rf *ResultsForest) processCompleted(resultID flow.Identifier) error {
	// first, ensure that the result ID is in the forest, otherwise the forest is in an inconsistent state
	container, found := rf.getContainer(resultID)
	if !found {
		return fmt.Errorf("result %s not found in forest", resultID)
	}

	// next, ensure that this result matches the latest persisted sealed result, otherwise
	// the forest is in an inconsistent state since persisting must be done sequentially
	// Note: In practice this means that we are expecting a strict sequentiality of events
	// `OnStateUpdated` when the next pipeline reaches `optimistic_sync.StateComplete`.
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
// A container is known to be on an abandoned fork if *any* of the following conditions is true:
//  1. its parent is abandoned
//  2. one of its siblings is sealed
//  3. its block conflicts with a finalized block
//
// In the case of 3, it is difficult to determine that the block conflicts without traversal.
// However, one of the result's siblings will eventually be sealed, at which point this result
// will be abandoned. Skipping the check here simplifies the logic, and forgoes the optimization
// of abandoning the fork early.
//
// NOT CONCURRENCY SAFE!
func (rf *ResultsForest) isAbandonedFork(container *ExecutionResultContainer) bool {
	// optimization: if the container is sealed, it can't be in an abandoned fork
	if container.ResultStatus() == ResultSealed {
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
		if sibling.ResultStatus() == ResultSealed {
			isAbandoned = true
			break
		}
	}

	return isAbandoned
}

// abandonFork recursively abandons a container and all its descendants.
// NOT CONCURRENCY SAFE!
func (rf *ResultsForest) abandonFork(container *ExecutionResultContainer) {
	container.Pipeline().Abandon()
	for child := range rf.iterateChildren(container.ResultID()) {
		rf.abandonFork(child)
	}
}

// ensureAdmissible enforces that the given candidate result satisfies the criteria
// to be included in the ResultsForest. As per specification:
//   - ResultsForest must *always* include the children (ancestry degree 1) of 𝓹.
//   - For a result 𝒓 with larger ancestry degree or (partially) unknown ancestry,
//     we include 𝒓 if and only if 𝒓.Level is in the closed interval [𝓹.Level, 𝓱].
//
// By limit the view range of results eligible to be stored in the forest, this function
// effectively prevents unbounded memory consumption on the happy path:
//   - Assume that the forest is only fed with results for _known_ certified blocks. This has no
//     performance implications in practise, but prevents flooding attacks by Execution Nodes
//     [ENs], publishing results for made-up blocks. A simple limited-size LRU cache can be used
//     to hold receipts in the rare case of blocks arriving late.
//   - There are two classes of byzantine flooding attacks possible:
//     1. Byzantine ENs publish results for made up blocks. This would only hit the LRU cache
//     discussed above, without reaching the ResultsForest.
//     2. Byzantine EN publishes many conflicting receipts and chunk data packs for a certified
//     block. This would be a slashable offence, though the attack could go on for quite some time,
//     during which the AN should ideally remain functional. At the moment, such receipts might
//     be stored in the ResultsForest, potentially leading to unbounded memory consumption.
//
// NOT CONCURRENCY SAFE!
//
// Expected error returns during normal operations:
//   - [ErrPrunedView]: if the result's view is below the pruning horizon (lowest view in the )
//   - [ErrMaxViewDeltaExceeded]: for results that are not direct children of the 𝙡𝙤𝙬𝙚𝙨𝙩
//     𝘀𝗲𝗮𝗹𝗲𝗱 𝗿𝗲𝘀𝘂𝗹𝘁 𝓹, if their view is more than `maxViewDelta` ahead of 𝓹.Level
//
// TODO: I think it would strengthen the encapsulation if performed the insertion into the Levelled Forest
//   - Document idempotency: repeated insertion of the same candidate is a noop -- this contract holds
//     _until_ the candidate falls below pruning threshold, at which point, [ErrPrunedView] is returned.
//   - the updated function could be called `safeForestInsert` or similar
func (rf *ResultsForest) ensureAdmissible(candidate *ExecutionResultContainer) error {
	candidateView := candidate.BlockView()

	// History cutoff - only results with views greater or equal are to this threshold are eligible for storage in ResultsForest
	historyCutoffView := rf.latestSealedResult.BlockView()
	if candidateView < historyCutoffView {
		return ErrPrunedView
	}

	// children of 𝓹, aka `lowestSealedResult`, must always be accepted by the forest
	parentID, parentView := candidate.Parent()
	if rf.lowestSealedResult.ResultID() == parentID {
		// Note: theoretically, we do not need to check that the view is consistent for the parent, because the LevelledForest already does
		// this. Specifically, the LevelledForest inspects the parent-child relationship based on IDs and then confirms that the view matches.
		// Otherwise, the LevelledForest errors when attempting to add the child vertex. Therefore, if the LevelledForest accepts the child,
		// we can be sure that the view is consistent with its parent. Nevertheless, by also checking here, our function `ensureAdmissible`
		// is self-contained and does not rely on other components for its correctness, while repeating the check has negligible cost.
		if rf.latestSealedResult.BlockView() != parentView {
			return fmt.Errorf("candidate reports view %d of its parent %v, while parent itself declares its view to be %d", parentView, parentID, rf.latestSealedResult.BlockView())
		}
		return nil // candidate result satisfies criteria for storage in ResultsForest
	}

	// We reject results whose view above 𝓱 = 𝓹.Level + maxViewDelta
	if candidateView > historyCutoffView+rf.maxViewDelta {
		return ErrMaxViewDeltaExceeded
	}

	return nil
}
