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
	"github.com/rs/zerolog/log"
)

var ForkedExecutionStateErr = fmt.Errorf("forked execution state detected") // sentinel error

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
//   * ForkedExecutionStateErr (sentinel error)
//     In case a fork in the execution state has been detected.
func (s *ExecStateForkSuppressor) Add(irSeal *flow.IncorporatedResultSeal) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
		if errors.Is(err, ForkedExecutionStateErr) {
			err := operation.RetryOnConflict(s.db.Update, operation.UpdateExecutionForkDetected(true))
			if err != nil {
				return false, fmt.Errorf("failed to update execution-fork-detected flag: %w", err)
			}
		}
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

func (s *ExecStateForkSuppressor) logInconsistentSeals(s1, s2 *flow.IncorporatedResultSeal) uint {
	sc1json, err := json.Marshal(irSeal)
	if err != nil {
		return nil, err
	}
	sc2json, err := json.Marshal(irSeal2)
	if err != nil {
		return nil, err
	}
	block, err := b.headers.ByBlockID(irSeal.Seal.BlockID)
	if err != nil {
		// not finding the block for a seal is a fatal, internal error: respective Execution Result should have been rejected by matching engine
		// we still print as much of the error message as we can about the inconsistent seals
		fmt.Printf("WARNING: multiple seals with different IDs for the same block %v: %s and %s\n", irSeal.Seal.BlockID, string(sc1json), string(sc2json))
		return nil, fmt.Errorf("could not retrieve block for seal: %w", err)
	}

	// matching engine only adds seals to the mempool that have final and initial state
	irSealInitialState, _ := irSeal.IncorporatedResult.Result.InitialStateCommit()
	irSealFinalState, _ := irSeal.IncorporatedResult.Result.FinalStateCommitment()
	irSeal2InitialState, _ := irSeal2.IncorporatedResult.Result.InitialStateCommit()
	irSeal2FinalState, _ := irSeal2.IncorporatedResult.Result.FinalStateCommitment()

	// check whether seals are inconsistent:
	if !bytes.Equal(irSealFinalState, irSeal2FinalState) || !bytes.Equal(irSealInitialState, irSeal2InitialState) {
		fmt.Printf("ERROR: inconsistent seals for the same block %v at height %d: %s and %s\n", irSeal.Seal.BlockID, block.Height, string(sc1json), string(sc2json))
		encounteredInconsistentSealsForSameBlock = true
	} else {
		fmt.Printf("WARNING: multiple seals with different IDs for the same block %v at height %d: %s and %s\n", irSeal.Seal.BlockID, block.Height, string(sc1json), string(sc2json))
	}

}

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

func setExecutionForkDetectedFlag(db *badger.DB) (bool, error) {

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

// consistenStateTransitions checks whether the execution results in the seals have matching state transitions
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
		return ForkedExecutionStateErr
	} else {
		log.Warn().Msg("seals with different ID but consistent state transition")
		return nil
	}
}

// ExecStateTransition represents an execution state transition from state `start` to state `end`
type ExecStateTransition struct {
	start flow.StateCommitment
	end   flow.StateCommitment
}

// NewExecStateTransition constructs a new state ExecStateTransition from the provided ExecutionResult.
// Enforces that result's initial and final state are non-empty
func NewExecStateTransition(result *flow.ExecutionResult) (*ExecStateTransition, error) {
	initialState, ok := result.InitialStateCommit()
	if !ok || len(initialState) < 1 {
		log.Error().Msg("execution receipt without InitialStateCommit received")
		return nil, engine.NewInvalidInputErrorf("execution result without InitialStateCommit: %x", result.ID())
	}
	finalState, ok := result.FinalStateCommitment()
	if !ok || len(finalState) < 1 {
		return nil, engine.NewInvalidInputErrorf("execution result without FinalStateCommit: %x", result.ID())
	}
	return &ExecStateTransition{
		start: initialState,
		end:   finalState,
	}, nil
}

// IsConsistent returns true if and only if irSeal has the same start and end state as this ExecStateTransition
func (t *ExecStateTransition) IsConsistent(result *flow.ExecutionResult) (bool, error) {
	initialState, ok := result.InitialStateCommit()
	if !ok || len(initialState) < 1 {
		log.Error().Msg("execution receipt without InitialStateCommit received")
		return false, engine.NewInvalidInputErrorf("execution result without InitialStateCommit: %x", result.ID())
	}
	finalState, ok := result.FinalStateCommitment()
	if !ok || len(finalState) < 1 {
		return false, engine.NewInvalidInputErrorf("execution result without FinalStateCommit: %x", result.ID())
	}
	return bytes.Equal(t.start, initialState) && bytes.Equal(t.end, initialState), nil
}
