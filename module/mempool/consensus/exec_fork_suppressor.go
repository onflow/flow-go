package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

var executionForkErr = fmt.Errorf("forked execution state detected") // sentinel error

// ExecForkSuppressor is a wrapper around a conventional mempool.IncorporatedResultSeals
// mempool. It implements the following mitigation strategy for execution forks:
//   * In case two conflicting results are considered sealable for the same block,
//     sealing should halt. Specifically, two results are considered conflicting,
//     if they differ in their start or end state.
//   * Even after a restart, the sealing should not resume.
//   * We rely on human intervention to resolve the conflict.
// The ExecForkSuppressor implements this mitigation strategy as follows:
//   * For each candidate seal inserted into the mempool, inspect the state
//     transition for the respective block.
//   * If this is the first seal for a block, store the seal as an archetype
//     for the state transition into the internal map `sealsForBlock`.
//   * If the mempool already knows about a state transition for a block,
//     and a second seal for the same block is inserted, check whether
//     the seal has the same state transition.
//   * If conflicting state transitions for the same block are detected,
//     ExecForkSuppressor sets an internal flag and thereafter
//     reports the mempool as empty, which will lead to the respective
//     consensus node not including any more seals.
//   * Evidence for an execution fork stored in a database (persisted across restarts).
// Implementation is concurrency safe.
type ExecForkSuppressor struct {
	mutex            sync.RWMutex
	seals            mempool.IncorporatedResultSeals
	sealsForBlock    map[flow.Identifier]sealSet // map BlockID -> set of IncorporatedResultSeal
	execForkDetected bool
	onExecFork       ExecForkActor
	db               *badger.DB
	log              zerolog.Logger
}

// sealSet is a set of seals; internally represented as a map from sealID -> to seal
type sealSet map[flow.Identifier]*flow.IncorporatedResultSeal

func NewExecStateForkSuppressor(onExecFork ExecForkActor, seals mempool.IncorporatedResultSeals, db *badger.DB, log zerolog.Logger) (*ExecForkSuppressor, error) {
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
		execForkDetected: execForkDetectedFlag,
		onExecFork:       onExecFork,
		db:               db,
		log:              log.With().Str("mempool", "ExecForkSuppressor").Logger(),
	}
	seals.RegisterEjectionCallbacks(wrapper.onEject)

	return &wrapper, nil
}

// onEject is the callback, which the wrapped mempool should call whenever it ejects an element
func (s *ExecForkSuppressor) onEject(entity flow.Entity) {
	// uncaught type assertion; should never panic as mempool.IncorporatedResultSeals only stores IncorporatedResultSeal
	irSeal := entity.(*flow.IncorporatedResultSeal)
	sealID := irSeal.ID()
	blockID := irSeal.Seal.BlockID
	log := s.log.With().
		Hex("seal_id", sealID[:]).
		Hex("block_id", blockID[:]).
		Logger()

	// CAUTION: potential edge case:
	// Upon adding a new seal, the ejector of the wrapped mempool decides to eject the element which was just added.
	// In this case, the ejected seal is _not_ the secondary index.
	//  (a) we don't have any seals for the respective block stored in the secondary index (yet).
	//  (b) the secondary index contains only one seal for the block, which
	//      is different than the seal just ejected
	set, found := s.sealsForBlock[irSeal.Seal.BlockID]
	if !found { // case (a)
		return
	}
	delete(set, irSeal.ID())
	if len(set) == 0 {
		delete(s.sealsForBlock, irSeal.Seal.BlockID)
	}
	log.Debug().Msg("ejected seal")
}

// Add adds the given seal to the mempool. Return value indicates whether or not seal was added to mempool.
// Error returns:
//   * engine.InvalidInputError (sentinel error)
//     In case a seal fails one of the required consistency checks;
func (s *ExecForkSuppressor) Add(newSeal *flow.IncorporatedResultSeal) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.execForkDetected {
		return false, nil
	}

	// STEP 1: ensure locally that newSeal's start and end state are non-empty values
	// This wrapper is a temporary safety layer; we check all conditions that are
	// required for its correct functioning locally, to not delegate safety-critical
	// implementation aspects to external components
	err := s.enforceValidStates(newSeal)
	if err != nil {
		return false, fmt.Errorf("invalid candidate seal: %w", err)
	}
	blockID := newSeal.Seal.BlockID

	// STEP 2: enforce that newSeal's state transition does not conflict with other stored seals for the same block
	otherSeals, found := s.sealsForBlock[blockID]
	if found {
		// already other seal for this block in mempool => compare consistency of results' state transitions
		otherSeal := getArbitraryElement(otherSeals) // cannot be nil, as otherSeals is guaranteed to always contain at least one element
		err := s.enforceConsistentStateTransitions(newSeal, otherSeal)
		if errors.Is(err, executionForkErr) {
			s.onExecFork([]*flow.IncorporatedResultSeal{newSeal, otherSeal})
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("state consistency check failed: %w", err)
		}
	} // no conflicting state transition for this block known

	// STEP 3: add newSeal to the wrapped mempool
	added, err := s.seals.Add(newSeal) // internally de-duplicates
	if err != nil {
		return added, fmt.Errorf("failed to add seal to wrapped mempool: %w", err)
	}
	if !added { // if underlying mempool did not accept the seal => nothing to do anymore
		return false, nil
	}

	// STEP 4: check whether wrapped mempool ejected the newSeal right away;
	// important to prevent memory leak
	newSealID := newSeal.ID()
	if _, exists := s.seals.ByID(newSealID); !exists {
		return added, nil
	}

	// STEP 4: add newSeal to secondary index of this wrapper
	// CAUTION: the following edge case needs to be considered:
	//  * the mempool only holds a single other seal (denominated as `otherSeal`) for this block
	//  * upon adding the new seal, the mempool might decide to eject otherSeal
	//  * during the ejection, we will delete the entire set from the `sealsForBlock`
	//    because at this time, it only held otherSeal, which was ejected
	// Therefore, the value for `found` in the line below might
	// be different than the value in the earlier call above.
	blockSeals, found := s.sealsForBlock[blockID]
	if !found {
		// no other seal for this block was in mempool before => create a set for the seals for this block
		blockSeals = make(sealSet)
		s.sealsForBlock[blockID] = blockSeals
	}
	blockSeals[newSealID] = newSeal
	return true, nil
}

// All returns all the IncorporatedResultSeals in the mempool
func (s *ExecForkSuppressor) All() []*flow.IncorporatedResultSeal {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.All()
}

// ByID returns an IncorporatedResultSeal by its ID
func (s *ExecForkSuppressor) ByID(identifier flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.ByID(identifier)
}

// Rem removes the IncorporatedResultSeal with id from the mempool
func (s *ExecForkSuppressor) Rem(id flow.Identifier) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	seal, found := s.seals.ByID(id)
	if found {
		s.seals.Rem(id)
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

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (s *ExecForkSuppressor) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection) {
	s.seals.RegisterEjectionCallbacks(callbacks...)
}

// enforceValidStates checks that seal has valid, non-empty, initial and final state.
// In case a seal fails the check, a detailed error message is logged and an
// engine.InvalidInputError (sentinel error) is returned.
func (s *ExecForkSuppressor) enforceValidStates(irSeal *flow.IncorporatedResultSeal) error {
	result := irSeal.IncorporatedResult.Result

	if _, ok := result.InitialStateCommit(); !ok {
		scjson, err := json.Marshal(irSeal)
		if err != nil {
			return err
		}
		s.log.Error().
			Str("seal", string(scjson)).
			Msg("seal's execution result has no InitialStateCommit")
		return engine.NewInvalidInputErrorf("seal's execution result has no InitialStateCommit: %x", result.ID())
	}

	if _, ok := result.FinalStateCommitment(); !ok {
		scjson, err := json.Marshal(irSeal)
		if err != nil {
			return err
		}
		s.log.Error().
			Str("seal", string(scjson)).
			Msg("seal's execution result has no FinalStateCommit")
		return engine.NewInvalidInputErrorf("seal's execution result has no FinalStateCommit: %x", result.ID())
	}

	return nil
}

// getArbitraryElement picks and returns an arbitrary element from the set. Returns nil, if set is empty.
func getArbitraryElement(set sealSet) *flow.IncorporatedResultSeal {
	for _, seal := range set {
		return seal
	}
	return nil
}

// enforceConsistentStateTransitions checks whether the execution results in the seals
// have matching state transitions. If a fork in the execution state is detected:
//   * wrapped mempool is cleared
//   * internal execForkDetected flag is ste to true
//   * the new value of execForkDetected is persisted to data base
// and executionForkErr (sentinel error) is returned
func (s *ExecForkSuppressor) enforceConsistentStateTransitions(irSeal, irSeal2 *flow.IncorporatedResultSeal) error {
	if irSeal.IncorporatedResult.Result.ID() == irSeal2.IncorporatedResult.Result.ID() {
		// happy case: candidate seals are for the same result
		return nil
	}
	// the results for the seals have different IDs (!)
	// => check whether initial and final state match in both seals

	// unsafe: we assume validity of states has been checked before
	irSeal1InitialState, _ := irSeal.IncorporatedResult.Result.InitialStateCommit()
	irSeal1FinalState, _ := irSeal.IncorporatedResult.Result.FinalStateCommitment()
	irSeal2InitialState, _ := irSeal2.IncorporatedResult.Result.InitialStateCommit()
	irSeal2FinalState, _ := irSeal2.IncorporatedResult.Result.FinalStateCommitment()

	if !bytes.Equal(irSeal1InitialState, irSeal2InitialState) || !bytes.Equal(irSeal1FinalState, irSeal2FinalState) {
		log.Error().Msg("inconsistent seals for the same block")
		s.seals.Clear()
		s.execForkDetected = true
		err := storeExecutionForkEvidence([]*flow.IncorporatedResultSeal{irSeal, irSeal2}, s.db)
		if err != nil {
			return fmt.Errorf("failed to update execution-fork-detected flag: %w", err)
		}
		return executionForkErr
	}
	log.Warn().Msg("seals with different ID but consistent state transition")
	return nil
}

// checkExecutionForkDetected checks the database whether evidence
// about an execution fork is stored. Returns the stored evidence.
func checkExecutionForkEvidence(db *badger.DB) ([]*flow.IncorporatedResultSeal, error) {
	var conflictingSeals []*flow.IncorporatedResultSeal
	err := db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveExecutionForkEvidence(&conflictingSeals)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			return nil // no evidence in data base; conflictingSeals is still nil slice
		}
		if err != nil {
			return fmt.Errorf("failed to load evidence whether or not an execution fork occured: %w", err)
		}
		return nil
	})
	return conflictingSeals, err
}

// storeExecutionForkEvidence stores the provided seals in the database
// as evidence for an execution fork.
func storeExecutionForkEvidence(conflictingSeals []*flow.IncorporatedResultSeal, db *badger.DB) error {
	err := operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		err := operation.InsertExecutionForkEvidence(conflictingSeals)(tx)
		if errors.Is(err, storage.ErrAlreadyExists) {
			// some evidence about execution fork already stored;
			// we only keep the first evidence => noting more to do
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to store evidence about execution fork: %w", err)
		}
		return nil
	})
	return err
}
