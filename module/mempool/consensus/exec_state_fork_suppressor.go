package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/rs/zerolog"
)

var ExecutionForkErr = fmt.Errorf("forked execution state detected") // sentinel error

// ExecStateForkSuppressor is a wrapper around a conventional mempool.IncorporatedResultSeals
// mempool. It implements the following mitigation strategy for execution forks:
//   * In case two conflicting results are considered sealable for the same block,
//     sealing should halt. Specifically, two results are considered conflicting,
//     if they differ in their start or end state.
//   * Even after a restart, the sealing should not resume.
//   * We rely on human intervention to resolve the conflict.
// The ExecStateForkSuppressor implements this mitigation strategy as follows:
//   * For each candidate seal inserted into the mempool, inspect the state
//     transition for the respective block.
//   * If this is the first seal for a block, store the state transition
//     proposed by the seal into an internal map `byBlockID`.
//   * If the mempool already knowns about a state transition for a block,
//     and a second seal for the same block is inserted, check whether
//     the seal has the same state transition.
//   * If conflicting state transitions for the same block are detected,
//     ExecStateForkSuppressor sets an internal flag and thereafter
//     reports the mempool as empty, which will lead to the respective
//     consensus node not including any more seals.
//   * The flag is stored in a database for persistence across restarts.
// Implementation is concurrency safe.
type ExecStateForkSuppressor struct {
	mutex            sync.RWMutex
	seals            mempool.IncorporatedResultSeals
	byBlockID        map[flow.Identifier]*flow.IncorporatedResultSeal
	execForkDetected bool
	db               *badger.DB
	log              zerolog.Logger
}

func NewExecStateForkSuppressor(seals mempool.IncorporatedResultSeals, db *badger.DB, log zerolog.Logger) (*ExecStateForkSuppressor, error) {
	flag, err := readExecutionForkDetectedFlag(db, false)
	if err != nil {
		return nil, fmt.Errorf("failed to interface with storage: %w", err)
	}

	return &ExecStateForkSuppressor{
		mutex:            sync.RWMutex{},
		seals:            seals,
		byBlockID:        make(map[flow.Identifier]*flow.IncorporatedResultSeal),
		execForkDetected: flag,
		db:               db,
		log:              log.With().Str("mempool", "ExecStateForkSuppressor").Logger(),
	}, nil
}

// Add adds the given seal to the mempool. Return value indicates whether or not
// seal was added to mempool.
// Error returns:
//   * engine.InvalidInputError (sentinel error)
//     In case a seal fails one of the required consistency checks;
//   * ExecutionForkErr (sentinel error)
//     In case a fork in the execution state has been detected.
func (s *ExecStateForkSuppressor) Add(irSeal *flow.IncorporatedResultSeal) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.execForkDetected {
		return false, ExecutionForkErr
	}

	err := s.enforceValidStates(irSeal)
	if err != nil {
		return false, fmt.Errorf("invalid candidate seal: %w", err)
	}
	blockID := irSeal.Seal.BlockID

	// check whether we already have a seal for the same block in the mempool:
	otherSeal, found := s.byBlockID[blockID]
	if found {
		// already other seal for this block in mempool => compare consistency of results' state transitions
		err := s.enforceConsistenStateTransitions(irSeal, otherSeal)
		if err != nil {
			return false, fmt.Errorf("failed to add candidate seal to mempool: %w", err)
		}
		// state transitions are consistent => add seal to wrapped mempool
	} else {
		// no other seal for this block in mempool => store seal as archetype result for block
		s.byBlockID[blockID] = irSeal
	}

	added, err := s.seals.Add(irSeal) // internally de-duplicates
	if err != nil {
		return added, fmt.Errorf("failed to add seal to wrapped mempool: %w", err)
	}
	return added, nil
}

// All returns all the IncorporatedResultSeals in the mempool
func (s *ExecStateForkSuppressor) All() []*flow.IncorporatedResultSeal {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.All()
}

// ByID returns an IncorporatedResultSeal by its ID
func (s *ExecStateForkSuppressor) ByID(identifier flow.Identifier) (*flow.IncorporatedResultSeal, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.ByID(identifier)
}

// Rem removes the IncorporatedResultSeal with id from the mempool
func (s *ExecStateForkSuppressor) Rem(id flow.Identifier) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	seal, found := s.seals.ByID(id)
	if found {
		s.seals.Rem(id)
		delete(s.byBlockID, seal.Seal.BlockID)
	}
	return found
}

// Size returns the number of items in the mempool
func (s *ExecStateForkSuppressor) Size() uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.Size()
}

// Limit returns the size limit of the mempool
func (s *ExecStateForkSuppressor) Limit() uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.seals.Limit()
}

// Clear removes all entities from the pool.
func (s *ExecStateForkSuppressor) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.seals.Clear()
}

// readExecutionForkDetectedFlag attempts to read the flag ExecutionForkDetected from the database.
// In case no value is persisted, the default value is written to the database.
func readExecutionForkDetectedFlag(db *badger.DB, defaultValue bool) (bool, error) {
	var flag bool
	err := operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		err := operation.RetrieveExecutionForkDetected(&flag)(tx)
		if errors.Is(err, storage.ErrNotFound) { // that no value was previously stored, which is expected on fist initialization
			err := operation.InsertExecutionForkDetected(defaultValue)(tx)
			if err != nil {
				return fmt.Errorf("error setting default value for execution-fork-detected flag: %w", err)
			}
			// happy case: flag was not stored and we have now successfully initialized it with the default value
			flag = defaultValue
			return nil
		} else if err != nil {
			return fmt.Errorf("error loading value for execution-fork-detected flag: %w", err)
		}
		return nil // happy case: flag was previously stored and we have successfully retrieved it
	})
	return flag, err
}

// enforceValidStates checks that seal has valid, non-empty, initial and final state.
// In case a seal fails the check, a detailed error message is logged and an
// engine.InvalidInputError (sentinel error) is returned.
func (s *ExecStateForkSuppressor) enforceValidStates(irSeal *flow.IncorporatedResultSeal) error {
	result := irSeal.IncorporatedResult.Result

	initialState, ok := result.InitialStateCommit()
	if !ok || len(initialState) < 1 {
		scjson, err := json.Marshal(irSeal)
		if err != nil {
			return err
		}
		s.log.Error().
			Str("seal", string(scjson)).
			Msg("seal's execution result has no InitialStateCommit")
		return engine.NewInvalidInputErrorf("seal's execution result has no InitialStateCommit: %x", result.ID())
	}

	finalState, ok := result.FinalStateCommitment()
	if !ok || len(finalState) < 1 {
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

// consistenStateTransitions checks whether the execution results in the seals have matching state transitions.
// If a fork in the execution state is detected:
//   * wrapped mempool is cleared
//   * internal execForkDetected flag is ste to true
//   * the new value of execForkDetected is persisted to data base
// and ExecutionForkErr (sentinel error) is returned
func (s *ExecStateForkSuppressor) enforceConsistenStateTransitions(irSeal1, irSeal2 *flow.IncorporatedResultSeal) error {
	if irSeal1.IncorporatedResult.Result.ID() == irSeal2.IncorporatedResult.Result.ID() {
		// happy case: candidate seals are for the same result
		return nil
	}
	// the results for the seals have different IDs
	// => check whether initial and final state match in both seals

	sc1json, err := json.Marshal(irSeal1)
	if err != nil {
		return fmt.Errorf("failed to marshal candidate seal to json: %w", err)
	}
	sc2json, err := json.Marshal(irSeal2)
	if err != nil {
		return fmt.Errorf("failed to marshal candidate seal to json: %w", err)
	}
	log := s.log.With().
		Str("seal_1", string(sc1json)).
		Str("seal_2", string(sc2json)).
		Logger()

	// unsafe: we assume validity of states has been checked before
	irSeal1InitialState, _ := irSeal1.IncorporatedResult.Result.InitialStateCommit()
	irSeal1FinalState, _ := irSeal1.IncorporatedResult.Result.FinalStateCommitment()
	irSeal2InitialState, _ := irSeal2.IncorporatedResult.Result.InitialStateCommit()
	irSeal2FinalState, _ := irSeal2.IncorporatedResult.Result.FinalStateCommitment()

	if !bytes.Equal(irSeal1InitialState, irSeal2InitialState) || !bytes.Equal(irSeal1FinalState, irSeal2FinalState) {
		log.Error().Msg("inconsistent seals for the same block")
		s.seals.Clear()
		s.execForkDetected = true
		err := operation.RetryOnConflict(s.db.Update, operation.UpdateExecutionForkDetected(true))
		if err != nil {
			return fmt.Errorf("failed to update execution-fork-detected flag: %w", err)
		}
		return ExecutionForkErr
	} else {
		log.Warn().Msg("seals with different ID but consistent state transition")
		return nil
	}
}
