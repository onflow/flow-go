package state

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	// NewView creates a new ready-only view at the given state commitment.
	NewView(flow.StateCommitment) *View
	// CommitDelta commits a register delta and returns the new state commitment.
	CommitDelta(*View) (flow.StateCommitment, []flow.StateRead, []flow.StateWrite, error)

	GetRegisters(flow.StateCommitment, []flow.RegisterID) ([]flow.RegisterValue, error)
	// GetChunkRegisters(flow.Identifier) (flow.Ledger, error)
	// Get all the read and writes of a chunk with proofs
	// GetChunkDataPack(flow.Identifier) (flow.ChunkDataPack, error)

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(flow.Identifier) (flow.StateCommitment, error)
	// PersistStateCommitment saves a state commitment by the given block ID.
	PersistStateCommitment(flow.Identifier, flow.StateCommitment) error

	// ChunkDataByChunkID returns the chunk data for the provided chunk ID.
	ChunkDataPackByChunkID(flow.Identifier) (*flow.ChunkDataPack, error)
	// PersistChunkData saves a chunk data by chunk ID.
	PersistChunkDataPack(*flow.ChunkDataPack) error
}

type state struct {
	ls             storage.Ledger
	commits        storage.Commits
	chunkDataPacks storage.ChunkDataPacks
}

// NewExecutionState returns a new execution state access layer for the given ledger storage.
func NewExecutionState(
	ls storage.Ledger,
	commits storage.Commits,
	chunkDataPacks storage.ChunkDataPacks,
) ExecutionState {
	return &state{
		ls:             ls,
		commits:        commits,
		chunkDataPacks: chunkDataPacks,
	}
}

func (s *state) NewView(commitment flow.StateCommitment) *View {

	return NewView(func(key flow.RegisterID) ([]byte, error) {
		values, err := s.ls.GetRegisters(
			[]flow.RegisterID{key},
			commitment,
		)
		if err != nil {
			return nil, fmt.Errorf("error getting register (%s) value at %x: %w", key, commitment, err)
		}

		if len(values) == 0 {
			return nil, nil
		}

		return values[0], nil
	}, commitment)
}

// RAMTIN: instead of delta we should pass the view object, and return the chunk data pack
func (s *state) CommitDelta(view *View) (flow.StateCommitment, []flow.StateRead, []flow.StateWrite, error) {
	ids, values := view.Delta().RegisterUpdates()
	commit, proofs, err := s.ls.UpdateRegistersWithProof(ids, values)
	if err != nil {
		return nil, nil, nil, err
	}
	stateWrites := make([]flow.StateWrite, len(ids))
	// TODO assert size of ids, values and proofs are the same
	for i, id := range ids {
		stateWrites = append(stateWrites, flow.StateWrite{
			RegisterID: id,
			Value:      values[i],
			Proof:      proofs[i],
		})
	}

	values, proofs, err = s.ls.GetRegistersWithProof(view.reads, view.state)
	if err != nil {
		return nil, nil, nil, err
	}
	stateReads := make([]flow.StateRead, len(view.reads))
	for i, id := range view.reads {
		stateReads = append(stateReads, flow.StateRead{RegisterID: id,
			Value: values[i],
			Proof: proofs[i]})
	}
	return commit, stateReads, stateWrites, nil
}

func (s *state) GetRegisters(
	commit flow.StateCommitment,
	registerIDs []flow.RegisterID,
) ([]flow.RegisterValue, error) {
	return s.ls.GetRegisters(registerIDs, commit)
}

// // RAMTIN THIS SHOULD CHANGE TO INCLUDE READ AND WRITES and RENAMED TO ChunkData
// func (s *state) GetChunkRegisters(chunkID flow.Identifier) (flow.Ledger, error) {
// 	chunkHeader, err := s.ChunkDataPackByChunkID(chunkID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to retrieve chunk data pack: %w", err)
// 	}

// 	registerIDs := chunkHeader.Reads()

// 	registerValues, err := s.GetRegisters(chunkHeader.StartState, registerIDs)

// 	// And Writes

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to retrieve chunk register values: %w", err)
// 	}

// 	l := make(flow.Ledger)

// 	for i, registerID := range registerIDs {
// 		l[string(registerID)] = registerValues[i]
// 	}

// 	return l, nil
// }

func (s *state) StateCommitmentByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return s.commits.ByID(blockID)
	// commit, err := s.commits.ByID(blockID)
	// if err != nil {
	// 	if errors.Is(err, storage.ErrNotFound) {
	// 		//TODO ? Shouldn't happen in MVP, in multi-node should query a state from other nodes
	// 		panic(fmt.Sprintf("storage commitment for id %v does not exist", blockID))
	// 	} else {
	// 		return nil, err
	// 	}
	// }
	// return commit, nil
}

func (s *state) PersistStateCommitment(blockID flow.Identifier, commit flow.StateCommitment) error {
	return s.commits.Store(blockID, commit)
}

func (s *state) ChunkDataPackByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	return s.chunkDataPacks.ByID(chunkID)
}

func (s *state) PersistChunkDataPack(c *flow.ChunkDataPack) error {
	return s.chunkDataPacks.Store(c)
}
