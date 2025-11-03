package state

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

type writesOnlySpockState struct {
	*storageState

	spockSecretHasher hash.Hasher

	getHasher func() hash.Hasher

	// NOTE: writesOnlySpockState is no longer accessible once Finalize is called.  We
	// can't support access after Finalize since spockSecretHasher.SumHash is
	// not idempotent.  Repeated calls to SumHash (without modifying the input)
	// may return different hashes.
	finalizedSpockSecret []byte
}

var _ state = &writesOnlySpockState{}

// newWritesOnlySpockState creates a new spock state.
// getHasher will be called to create a new hasher for the spock state and each child state
func newWritesOnlySpockState(base snapshot.StorageSnapshot, getHasher func() hash.Hasher) *writesOnlySpockState {
	return &writesOnlySpockState{
		storageState:      newStorageState(base),
		spockSecretHasher: getHasher(),
		getHasher:         getHasher,
	}
}

func (state *writesOnlySpockState) NewChild() state {
	return &writesOnlySpockState{
		storageState:      state.storageState.NewChild(),
		spockSecretHasher: state.getHasher(),
		getHasher:         state.getHasher,
	}
}

func (state *writesOnlySpockState) Finalize() *snapshot.ExecutionSnapshot {
	if state.finalizedSpockSecret == nil {
		state.finalizedSpockSecret = state.spockSecretHasher.SumHash()
	}

	snapshot := state.storageState.Finalize()
	snapshot.SpockSecret = state.finalizedSpockSecret
	return snapshot
}

func (state *writesOnlySpockState) Merge(snapshot *snapshot.ExecutionSnapshot) error {
	if state.finalizedSpockSecret != nil {
		return fmt.Errorf("cannot Merge on a finalized state")
	}

	_, err := state.spockSecretHasher.Write(mergeMarker)
	if err != nil {
		return fmt.Errorf("merge SPoCK failed: %w", err)
	}

	_, err = state.spockSecretHasher.Write(snapshot.SpockSecret)
	if err != nil {
		return fmt.Errorf("merge SPoCK failed: %w", err)
	}

	return state.storageState.Merge(snapshot)
}

func (state *writesOnlySpockState) Set(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	if state.finalizedSpockSecret != nil {
		return fmt.Errorf("cannot Set on a finalized state")
	}

	_, err := state.spockSecretHasher.Write(setMarker)
	if err != nil {
		return fmt.Errorf("set SPoCK failed: %w", err)
	}

	idBytes := id.Bytes()

	// Note: encoding the register id / value length as part of spock hash
	// to prevent string injection attacks.
	err = binary.Write(
		state.spockSecretHasher,
		binary.LittleEndian,
		int32(len(idBytes)))
	if err != nil {
		return fmt.Errorf("set SPoCK failed: %w", err)
	}

	_, err = state.spockSecretHasher.Write(idBytes)
	if err != nil {
		return fmt.Errorf("set SPoCK failed: %w", err)
	}

	err = binary.Write(
		state.spockSecretHasher,
		binary.LittleEndian,
		int32(len(value)))
	if err != nil {
		return fmt.Errorf("set SPoCK failed: %w", err)
	}

	_, err = state.spockSecretHasher.Write(value)
	if err != nil {
		return fmt.Errorf("set SPoCK failed: %w", err)
	}

	return state.storageState.Set(id, value)
}

func (state *writesOnlySpockState) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	if state.finalizedSpockSecret != nil {
		return nil, fmt.Errorf("cannot Get on a finalized state")
	}

	// NOTE: Reads do not affect the SPoCK hash

	return state.storageState.Get(id)
}

func (state *writesOnlySpockState) DropChanges() error {
	if state.finalizedSpockSecret != nil {
		return fmt.Errorf("cannot DropChanges on a finalized state")
	}

	_, err := state.spockSecretHasher.Write(dropChangesMarker)
	if err != nil {
		return fmt.Errorf("drop changes SPoCK failed: %w", err)
	}

	return state.storageState.DropChanges()
}

func (state *writesOnlySpockState) readSetSize() int {
	return state.storageState.readSetSize()
}

func (state *writesOnlySpockState) interimReadSet(
	accumulator map[flow.RegisterID]struct{},
) {
	state.storageState.interimReadSet(accumulator)
}
