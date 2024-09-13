package consensus

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// ExecForkSuppressor is a wrapper around a conventional mempool.IncorporatedResultSeals
// mempool. It implements the following mitigation strategy for execution forks:
//   - In case two conflicting results are considered sealable for the same block,
//     sealing should halt. Specifically, two results are considered conflicting,
//     if they differ in their start or end state.
//   - Even after a restart, the sealing should not resume.
//   - We rely on human intervention to resolve the conflict.
//
// The ExecForkSuppressor implements this mitigation strategy as follows:
//   - For each candidate seal inserted into the mempool, indexes seal
//     by respective blockID, storing all seals in the internal map `sealsForBlock`.
//   - Whenever client perform any query, we check if there are conflicting seals.
//   - We pick first seal available for a block and check whether
//     the seal has the same state transition as other seals included for same block.
//   - If conflicting state transitions for the same block are detected,
//     ExecForkSuppressor sets an internal flag and thereafter
//     reports the mempool as empty, which will lead to the respective
//     consensus node not including any more seals.
//   - Evidence for an execution fork stored in a database (persisted across restarts).
//
// Implementation is concurrency safe.
type ExecForkSuppressor struct {
	mutex            sync.RWMutex
	seals            mempool.IncorporatedResultSeals
	sealsForBlock    map[flow.Identifier]sealSet             // map BlockID -> set of IncorporatedResultSeal
	byHeight         map[uint64]map[flow.Identifier]struct{} // map height -> set of executed block IDs at height
	lowestHeight     uint64
	execForkDetected atomic.Bool
	onExecFork       ExecForkActor
	db               *pebble.DB
	log              zerolog.Logger
}

var _ mempool.IncorporatedResultSeals = (*ExecForkSuppressor)(nil)

// sealSet is a set of seals; internally represented as a map from sealID -> to seal
type sealSet map[flow.Identifier]*flow.IncorporatedResultSeal

// sealsList is a list of seals
type sealsList []*flow.IncorporatedResultSeal

func NewExecStateForkSuppressor(seals mempool.IncorporatedResultSeals, onExecFork ExecForkActor, db *pebble.DB, log zerolog.Logger) (*ExecForkSuppressor, error) {
	conflictingSeals, err := checkExecutionForkEvidence(db)
	if err != nil {
		return nil, fmt.Errorf("failed to interface with storage: %w", err)
	}
	execForkDetectedFlag := len(conflictingSeals) != 0
	if execForkDetectedFlag {
		onExecFork(conflictingSeals)
	}

	wrapper := ExecForkSuppressor{
		mutex:            sync.RWMutex{},
		seals:            seals,
		sealsForBlock:    make(map[flow.Identifier]sealSet),
		byHeight:         make(map[uint64]map[flow.Identifier]struct{}),
		execForkDetected: *atomic.NewBool(execForkDetectedFlag),
		onExecFork:       onExecFork,
		db:               db,
		log:              log.With().Str("mempool", "ExecForkSuppressor").Logger(),
	}

	return &wrapper, nil
}

// Add adds the given seal to the mempool. Return value indicates whether seal was added to the mempool.
// Internally indexes every added seal by blockID. Expects that underlying mempool never eject items.
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

	// This mempool allows adding multiple seals for same blockID even if they have different state transition.
	// When builder logic tries to query such seals we will check whenever we have an execution fork. The main reason for
	// detecting forks at query time(not at adding time) is ability to add extra logic in underlying mempools. For instance
	// we could filter seals comming from underlying mempool by some criteria.

	// STEP 2: add newSeal to the wrapped mempool
	added, err := s.seals.Add(newSeal) // internally de-duplicates
	if err != nil {
		return added, fmt.Errorf("failed to add seal to wrapped mempool: %w", err)
	}
	if !added { // if underlying mempool did not accept the seal => nothing to do anymore
		return false, nil
	}

	// STEP 3: add newSeal to secondary index of this wrapper
	// CAUTION: We expect that underlying mempool NEVER ejects seals because it breaks liveness.
	blockSeals, found := s.sealsForBlock[blockID]
	if !found {
		// no other seal for this block was in mempool before => create a set for the seals for this block
		blockSeals = make(sealSet)
		s.sealsForBlock[blockID] = blockSeals
	}
	blockSeals[newSeal.ID()] = newSeal

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

	// index seals retrieved from underlying mepool by blockID to check
	// for conflicting seals
	sealsByBlockID := make(map[flow.Identifier]sealsList, 0)
	for _, seal := range seals {
		sealsPerBlock := sealsByBlockID[seal.Seal.BlockID]
		sealsByBlockID[seal.Seal.BlockID] = append(sealsPerBlock, seal)
	}

	// check for conflicting seals
	return s.filterConflictingSeals(sealsByBlockID)
}

// ByID returns an IncorporatedResultSeal by its ID.
// The IncorporatedResultSeal's ID is the same as IncorporatedResult's ID,
// so this call essentially is to find the seal for the incorporated result in the mempool.
// Note: This call might crash if the block of the seal has multiple seals in mempool for conflicting
// incorporated results. Usually the builder will call this method to find a seal for an incorporated
// result, so the builder might crash if multiple conflicting seals exist.
func (s *ExecForkSuppressor) ByID(identifier flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	s.mutex.RLock()
	seal, found := s.seals.ByID(identifier)
	// if we haven't found seal in underlying storage - exit early
	if !found {
		s.mutex.RUnlock()
		return seal, found
	}
	sealsForBlock := s.sealsForBlock[seal.Seal.BlockID]
	// if there are no other seals for this block previously seen - then no possible execution forks
	if len(sealsForBlock) == 1 {
		s.mutex.RUnlock()
		return seal, true
	}
	// convert map into list
	var sealsPerBlock sealsList
	for _, otherSeal := range sealsForBlock {
		sealsPerBlock = append(sealsPerBlock, otherSeal)
	}
	s.mutex.RUnlock()

	// check for conflicting seals
	seals := s.filterConflictingSeals(map[flow.Identifier]sealsList{seal.Seal.BlockID: sealsPerBlock})
	if len(seals) == 0 {
		return nil, false
	}
	return seals[0], true
}

// Remove removes the IncorporatedResultSeal with id from the mempool
func (s *ExecForkSuppressor) Remove(id flow.Identifier) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	seal, found := s.seals.ByID(id)
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
	s.sealsForBlock = make(map[flow.Identifier]sealSet)
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

// enforceConsistentStateTransitions checks whether the execution results in the seals
// have matching state transitions. If a fork in the execution state is detected:
//   - wrapped mempool is cleared
//   - internal execForkDetected flag is ste to true
//   - the new value of execForkDetected is persisted to data base
//
// and executionForkErr (sentinel error) is returned
// The function assumes the execution results in the seals have a non-zero number of chunks.
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

// checkExecutionForkDetected checks the database whether evidence
// about an execution fork is stored. Returns the stored evidence.
func checkExecutionForkEvidence(db *pebble.DB) ([]*flow.IncorporatedResultSeal, error) {
	var conflictingSeals []*flow.IncorporatedResultSeal
	err := operation.RetrieveExecutionForkEvidence(&conflictingSeals)(db)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil // no evidence in data base; conflictingSeals is still nil slice
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load evidence whether or not an execution fork occured: %w", err)
	}
	return conflictingSeals, nil
}

// storeExecutionForkEvidence stores the provided seals in the database
// as evidence for an execution fork.
func storeExecutionForkEvidence(conflictingSeals []*flow.IncorporatedResultSeal, db *pebble.DB) error {
	err := operation.InsertExecutionForkEvidence(conflictingSeals)(db)
	if errors.Is(err, storage.ErrAlreadyExists) {
		// some evidence about execution fork already stored;
		// we only keep the first evidence => noting more to do
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to store evidence about execution fork: %w", err)
	}
	return nil
}

// filterConflictingSeals performs filtering of provided seals by checking if there are conflicting seals for same block.
// For every block we check if first seal has same state transitions as others. Multiple seals for same block are allowed
// but their state transitions should be the same. Upon detecting seal with inconsistent state transition we will clear our mempool,
// stop accepting new seals and querying old seals and store execution fork evidence into DB. Creator of mempool will be notified
// by callback.
func (s *ExecForkSuppressor) filterConflictingSeals(sealsByBlockID map[flow.Identifier]sealsList) sealsList {
	var result sealsList
	for _, sealsInBlock := range sealsByBlockID {
		if len(sealsInBlock) > 1 {
			// enforce that newSeal's state transition does not conflict with other stored seals for the same block
			// already other seal for this block in mempool => compare consistency of results' state transitions
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
				err := storeExecutionForkEvidence(conflictingSeals, s.db)
				if err != nil {
					panic("failed to store execution fork evidence")
				}
				s.onExecFork(conflictingSeals)
				return nil
			}
		}
		result = append(result, sealsInBlock...)
	}
	return result
}
