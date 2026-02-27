package consensus

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

// ExecForkSuppressor is a wrapper around a conventional [mempool.IncorporatedResultSeals]
// mempool. It implements the following mitigation strategy for execution forks:
//   - In case two conflicting results are considered sealable for the same block,
//     sealing should halt. Specifically, two results are considered conflicting,
//     if they differ in their start or end state.
//   - Even after a restart, the sealing should not resume.
//   - We rely on human intervention to resolve the conflict.
//
// The ExecForkSuppressor implements this mitigation strategy as follows:
//   - Let 𝓈 be a seal in the mempool. With 𝓈.BlockID we denote the ID of the block whose result
//     would be sealed by 𝓈. The ID of the incorporated result the seal commits to is denoted by 𝓈.IncorporatedResultID.
//     For each seal 𝓈 successfully added to the underlying mempool, we store the following entries
//     in our internal maps:
//     `sealsForBlock[𝓈.BlockID][𝓈.IncorporatedResultID] = struct{}{}`
//     `byHeight[𝓈.Header.Height][𝓈.BlockID] = struct{}{}`
//   - Whenever a client performs a query, we check if there are conflicting seals.
//   - We pick the first seal available for a block and check whether
//     the seal has the same state transition as other seals available for the same block.
//   - If conflicting state transitions for the same block are detected,
//     ExecForkSuppressor sets an internal flag and thereafter
//     reports the mempool as empty, which will lead to the respective
//     consensus node not including any more seals.
//   - Evidence for an execution fork is stored in a database (persisted across restarts).
//
// In the mature protocol, there will be mechanisms that prevent some seals from being created in the first place,
// but some of those mechanisms are not yet implemented. In place of the mature solution (seals for results
// with insufficient reliability never being added to the mempool), we work with heuristic filters preventing
// some of the receipts in the mempool from being retrieved. This is fine for now, because all ENs are operated
// by vetted partners. We make a deliberate design choice: the wrappers around the core mempool contain all the
// algorithmic shortcuts necessary to ensure correctness until we have the mature solution in place. Unfortunately,
// this design choice induces the following subtlety:
//
// IMPORTANT: For each block, `sealsForBlock` tracks all IncorporatedResult IDs for which seals have been
// forwarded to the underlying pool (see [potentiallySealableResults]). This is a SUPERSET of IncorporatedResults
// whose seals the underlying pool considers includable: the lower-level wrappers may hold back seals that don't
// meet their inclusion criteria (e.g., requiring at least 2 execution receipts from different ENs).
// Hence, the ExecForkSuppressor must NOT use `sealsForBlock` directly to determine conflicting seals. Instead,
// at query time, it verifies each candidate through the underlying pool, so that only actually-includable seals
// are compared for fork detection.
//
// Implementation is concurrency safe.
type ExecForkSuppressor struct {
	mutex                 sync.RWMutex
	seals                 mempool.IncorporatedResultSeals
	execForkDetected      atomic.Bool
	onExecFork            ExecForkActor
	execForkEvidenceStore storage.ExecutionForkEvidence
	lockManager           storage.LockManager
	log                   zerolog.Logger

	// sealsForBlock is a set of sets. Formally, it maps: BlockID -> set of IncorporatedResult IDs. Intuitively, for
	// each block that we see a seal for, we memorize the set of _all_ incorporated results that those seals pertain to.
	// `byHeight` and `lowestHeight` are used to prune `sealsForBlock` by height.
	sealsForBlock map[flow.Identifier]potentiallySealableResults // BlockID -> set of IncorporatedResult IDs (superset of wrapped pool, see struct docs)
	byHeight      map[uint64]map[flow.Identifier]struct{}        // map height -> set of executed block IDs at height
	lowestHeight  uint64
}

var _ mempool.IncorporatedResultSeals = (*ExecForkSuppressor)(nil)

// potentiallySealableResults is a set of IDs of IncorporatedResults that are potentially sealable.
// It is a SUPERSET of the IncorporatedResults that the wrapped pool considers sealable (see struct-level documentation).
// CAUTION: some of these seals might be held back from inclusion by the lower-level disaster prevention heuristics
// but they are still stored in the ExecForkSuppressor because the seals pass through the ExecForkSuppressor before
// being added to the underlying mempool.
//
// We intentionally store only the Incorporated Result IDs (not seal pertaining to them) to structurally enforce that all
// seal lookups go through the wrapped mempool, which may apply inclusion conditions.
type potentiallySealableResults map[flow.Identifier]struct{}

// sealsList is a list of seals
type sealsList []*flow.IncorporatedResultSeal

func NewExecStateForkSuppressor(
	seals mempool.IncorporatedResultSeals,
	onExecFork ExecForkActor,
	db storage.DB,
	lockManager storage.LockManager,
	log zerolog.Logger,
) (*ExecForkSuppressor, error) {
	executionForkEvidenceStore := store.NewExecutionForkEvidence(db)

	conflictingSeals, err := executionForkEvidenceStore.Retrieve()
	if err != nil {
		return nil, fmt.Errorf("failed to interface with storage: %w", err)
	}

	execForkDetectedFlag := len(conflictingSeals) != 0
	if execForkDetectedFlag {
		onExecFork(conflictingSeals)
	}

	wrapper := ExecForkSuppressor{
		mutex:                 sync.RWMutex{},
		seals:                 seals,
		sealsForBlock:         make(map[flow.Identifier]potentiallySealableResults),
		byHeight:              make(map[uint64]map[flow.Identifier]struct{}),
		execForkDetected:      *atomic.NewBool(execForkDetectedFlag),
		onExecFork:            onExecFork,
		execForkEvidenceStore: executionForkEvidenceStore,
		lockManager:           lockManager,
		log:                   log.With().Str("mempool", "ExecForkSuppressor").Logger(),
	}

	return &wrapper, nil
}

// Add forwards the given seal to the underlying mempool, with the return value indicating whether seal was accepted
// by the underlying mempool. For every `newSeal` that is accepted, we record:
//   - for block `newSeal.Seal.BlockID` add ` newSeal.IncorporatedResult.ID()` to the set of incorporated results
//     for that block, which we have seen seals for.
//     IMPORTANT: this set is a SUPERSET of the seals that the wrapped pool considers includable (see struct-level documentation).
//   - cache the block height in `byHeight` index, to enable later pruning of `sealsForBlock` by height.
//
// Error returns:
//   - engine.InvalidInputError (sentinel error)
//     In case a seal fails one of the required consistency checks;
func (s *ExecForkSuppressor) Add(newSeal *flow.IncorporatedResultSeal) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.execForkDetected.Load() {
		return false, nil
	}

	if newSeal.Header.Height < s.lowestHeight {
		return false, nil
	}

	// STEP 1: ensure locally that newSeal's chunks are non zero, which means
	// that the new seal contains start and end state values.
	// This wrapper is a temporary safety layer; we check all conditions that are
	// required for its correct functioning locally, to not delegate safety-critical
	// implementation aspects to external components
	err := s.enforceValidChunks(newSeal)
	if err != nil {
		return false, fmt.Errorf("invalid candidate seal: %w", err)
	}
	blockID := newSeal.Seal.BlockID

	// STEP 2: add newSeal to the wrapped mempool
	//
	// IMPORTANT: Formally, seals pertain to *Incorporated Results*, not blocks. Typically, we have a block B and all ENs publishing
	// the same result r[B] for block B. Consensus nodes then record r[B] in the child blocks of B - exactly once in each fork (simplified).
	// So just by the main chain forking, there can be multiple valid incorporated results for the same block. Note that we treat all
	// seals for the same block as equivalent (they may differ in which verifiers signed). In summary:
	//                            block (one) <--> (many) incorporated results
	//      and     incorporated result (one) <--> single seal as representative of equivalence class (mempool drops duplicates)
	//
	// By necessity of the mature protocol, the core mempool allows adding multiple seals for the same blockID but is oblivious to
	// whether they have different state transitions. This is because:
	// In the mature protocol, during severe network partitions, chunks may get temporarily lost and
	// execution nodes may become temporarily unreachable. At first, this can be indistinguishable from a byzantine attack
	// where a collector cluster withholds a collection. The network will proceed and attempt to restore liveness of execution by declaring
	// the collection as lost, allowing the remaining ENs to continue without it. If the network partition resolves at this point,
	// two valid seals might temporarily co-exist. Choosing either at random would be valid for the mature protocol, but *not* for
	// now, where forks are likely still just code bugs in the ENs.
	//
	// On the happy path, incorporated results for the same block all commit to the same final state. In contrast, accidental
	// execution forks (due to bugs) typically differ in the final state. Hence we use this as a simplified heuristic, assuming
	// different events or metadata necessitate different end states. Fork detection is deferred to query time
	// (not add time) because the wrapped mempool may apply additional inclusion conditions that change over time.
	// For instance, the wrapped pool might only consider a seal includable once sufficient execution receipts exist.
	// Fork detection should only trigger for seals that are actually includable.
	//
	added, err := s.seals.Add(newSeal) // internally de-duplicates
	if err != nil {
		return added, fmt.Errorf("failed to add seal to wrapped mempool: %w", err)
	}
	if !added { // if underlying mempool did not accept the seal => nothing to do anymore
		return false, nil
	}

	// STEP 3: record `newSeal.IncorporatedResultID()` in sealsForBlock.
	// IMPORTANT: sealsForBlock is a SUPERSET of IncorporatedResults the wrapped pool considers includable (see struct-level documentation).
	irIDs, found := s.sealsForBlock[blockID]
	if !found {
		// no other seal for this block was in mempool before => create a set for the seals for this block
		irIDs = make(potentiallySealableResults)
		s.sealsForBlock[blockID] = irIDs
	}
	irIDs[newSeal.IncorporatedResultID()] = struct{}{}

	// cache block height to prune additional index by height
	blocksAtHeight, found := s.byHeight[newSeal.Header.Height]
	if !found {
		blocksAtHeight = make(map[flow.Identifier]struct{})
		s.byHeight[newSeal.Header.Height] = blocksAtHeight
	}
	blocksAtHeight[blockID] = struct{}{}

	return true, nil
}

// All returns all the IncorporatedResultSeals in the mempool.
// Note: This call might crash if the block of the seal has multiple seals in mempool for conflicting
// incorporated results.
func (s *ExecForkSuppressor) All() []*flow.IncorporatedResultSeal {
	s.mutex.RLock()
	seals := s.seals.All()
	s.mutex.RUnlock()

	// index seals retrieved from underlying mempool by blockID to check for conflicting seals
	sealsByBlockID := make(map[flow.Identifier]sealsList, 0)
	for _, seal := range seals {
		sealsPerBlock := sealsByBlockID[seal.Seal.BlockID]
		sealsByBlockID[seal.Seal.BlockID] = append(sealsPerBlock, seal)
	}

	// check for conflicting seals
	return s.filterConflictingSeals(sealsByBlockID)
}

// Get returns an IncorporatedResultSeal by IncorporatedResult's ID.
// The wrapped pool's Get is used as the source of truth: it only returns seals satisfying
// the pool's inclusion conditions (e.g., sufficient execution receipts).
// For fork detection, we retrieve candidate seal IDs for the same block from sealsForBlock,
// then filter each through the wrapped pool to obtain only includable seals for conflict checking.
// Note: This call might crash if the block of the seal has multiple includable seals in
// mempool for conflicting incorporated results.
func (s *ExecForkSuppressor) Get(identifier flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	s.mutex.RLock()
	seal, found := s.seals.Get(identifier)
	// if we haven't found seal in underlying storage - exit early
	if !found {
		s.mutex.RUnlock()
		return seal, found
	}
	irIDs := s.sealsForBlock[seal.Seal.BlockID]
	if len(irIDs) == 1 {
		// only one IncorporatedResult known for this block => no possible execution fork
		s.mutex.RUnlock()
		return seal, true
	}
	// Multiple IncorporatedResults recorded for this block, the seals for some of which might not qualify yet for
	// inclusion and may be withheld by the lower-level wrappers. We limit our fork detection to incorporated results
	// whose seals actually qualify for inclusion ; we don't want to trigger on forks, whose sealing is suppressed
	// by lower-level wrappers.
	var sealsPerBlock sealsList
	for id := range irIDs {
		if candidateSeal, ok := s.seals.Get(id); ok {
			sealsPerBlock = append(sealsPerBlock, candidateSeal)
		}
	}
	s.mutex.RUnlock()

	// check for conflicting seals
	seals := s.filterConflictingSeals(map[flow.Identifier]sealsList{seal.Seal.BlockID: sealsPerBlock})
	if len(seals) == 0 {
		return nil, false
	}
	return seals[0], true
}

// Remove removes the IncorporatedResultSeal by IncorporatedResult ID from the mempool.
func (s *ExecForkSuppressor) Remove(id flow.Identifier) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	seal, found := s.seals.Get(id)
	if found {
		s.seals.Remove(id)
		set, found := s.sealsForBlock[seal.Seal.BlockID]
		if !found {
			// In the current implementation, this cannot happen, as every entity in the mempool is also contained in sealsForBlock.
			// we nevertheless perform this sanity check here, to catch future inconsistent code modifications
			s.log.Fatal().Msg("inconsistent state detected: seal not in secondary index")
		}
		if len(set) > 1 {
			delete(set, id)
		} else {
			delete(s.sealsForBlock, seal.Seal.BlockID)
		}
	}
	return found
}

// Size returns the number of items in the mempool
func (s *ExecForkSuppressor) Size() uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.Size()
}

// Limit returns the size limit of the mempool
func (s *ExecForkSuppressor) Limit() uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.Limit()
}

// Clear removes all entities from the pool.
// The wrapper clears the internal state as well as its local (additional) state.
func (s *ExecForkSuppressor) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sealsForBlock = make(map[flow.Identifier]potentiallySealableResults)
	s.seals.Clear()
}

// PruneUpToHeight remove all seals for blocks whose height is strictly
// smaller that height. Note: seals for blocks at height are retained.
func (s *ExecForkSuppressor) PruneUpToHeight(height uint64) error {
	err := s.seals.PruneUpToHeight(height)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.sealsForBlock) == 0 {
		s.lowestHeight = height
		return nil
	}

	// Optimization: if there are less height in the index than the height range to prune,
	// range to prune, then just go through each seal.
	// Otherwise, go through each height to prune.
	if uint64(len(s.byHeight)) < height-s.lowestHeight {
		for h := range s.byHeight {
			if h < height {
				s.removeByHeight(h)
			}
		}
	} else {
		for h := s.lowestHeight; h < height; h++ {
			s.removeByHeight(h)
		}
	}

	return nil
}

func (s *ExecForkSuppressor) removeByHeight(height uint64) {
	for blockID := range s.byHeight[height] {
		delete(s.sealsForBlock, blockID)
	}
	delete(s.byHeight, height)
}

// enforceValidChunks checks that seal has valid non-zero number of chunks.
// In case a seal fails the check, a detailed error message is logged and an
// engine.InvalidInputError (sentinel error) is returned.
func (s *ExecForkSuppressor) enforceValidChunks(irSeal *flow.IncorporatedResultSeal) error {
	result := irSeal.IncorporatedResult.Result

	if !result.ValidateChunksLength() {
		scjson, errjson := json.Marshal(irSeal)
		if errjson != nil {
			return errjson
		}
		s.log.Error().
			Str("seal", string(scjson)).
			Msg("seal's execution result has no chunks")
		return engine.NewInvalidInputErrorf("seal's execution result has no chunks: %x", result.ID())
	}
	return nil
}

// hasConsistentStateTransitions checks whether the state transitions of the two seals are consistent.
// Two seals have consistent state transitions iff they reference the same initial and final state.
// Pre-requisite: execution results in both seals have a non-zero number of chunks.
func hasConsistentStateTransitions(irSeal, irSeal2 *flow.IncorporatedResultSeal) bool {
	if irSeal.IncorporatedResult.Result.ID() == irSeal2.IncorporatedResult.Result.ID() {
		// happy case: candidate seals are for the same result
		return true
	}
	// the results for the seals have different IDs (!)
	// => check whether initial and final state match in both seals

	// unsafe: we assume validity of chunks has been checked before
	irSeal1InitialState, _ := irSeal.IncorporatedResult.Result.InitialStateCommit()
	irSeal1FinalState, _ := irSeal.IncorporatedResult.Result.FinalStateCommitment()
	irSeal2InitialState, _ := irSeal2.IncorporatedResult.Result.InitialStateCommit()
	irSeal2FinalState, _ := irSeal2.IncorporatedResult.Result.FinalStateCommitment()

	if irSeal1InitialState != irSeal2InitialState || irSeal1FinalState != irSeal2FinalState {
		log.Error().Msg("inconsistent seals for the same block")
		return false
	}
	log.Warn().Msg("seals with different ID but consistent state transition")
	return true
}

// filterConflictingSeals checks the provided seals for conflicting state transitions per block.
// CAUTION: All input seals must be eligible for including (i.e. not held back by lower-level mempool wrappers).
// Multiple seals for the same block are allowed as long as their state transitions are consistent.
// Upon detecting an inconsistent state transition, the mempool is cleared, the execForkDetected flag is set,
// evidence is persisted to the DB, and the onExecFork callback is invoked.
func (s *ExecForkSuppressor) filterConflictingSeals(sealsByBlockID map[flow.Identifier]sealsList) sealsList {
	var result sealsList
	for _, sealsInBlock := range sealsByBlockID {
		if len(sealsInBlock) > 1 {
			// check whether sealed results all commit to the same state transition
			var conflictingSeals sealsList
			candidateSeal := sealsInBlock[0]
			for _, otherSeal := range sealsInBlock[1:] {
				if !hasConsistentStateTransitions(candidateSeal, otherSeal) {
					conflictingSeals = append(conflictingSeals, otherSeal)
				}
			}
			// check if inconsistent state transition detected
			if len(conflictingSeals) > 0 {
				s.execForkDetected.Store(true)
				s.Clear()
				conflictingSeals = append(sealsList{candidateSeal}, conflictingSeals...)

				// Acquire lock and store execution fork evidence
				err := storage.WithLock(s.lockManager, storage.LockInsertExecutionForkEvidence, func(lctx lockctx.Context) error {
					return s.execForkEvidenceStore.StoreIfNotExists(lctx, conflictingSeals)
				})
				if err != nil {
					s.log.Fatal().Msg("failed to store execution fork evidence")
				}
				s.onExecFork(conflictingSeals)
				return nil
			}
		}
		result = append(result, sealsInBlock...)
	}
	return result
}
