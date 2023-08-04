package state

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// Note: encoding the operation type as part of the spock hash
	// prevents operation injection/substitution attacks.
	getMarker         = []byte("1")
	setMarker         = []byte("2")
	dropChangesMarker = []byte("3")
	mergeMarker       = []byte("4")
)

type spockState struct {
	*storageState

	spockSecretHasher hash.Hasher

	// NOTE: spockState is no longer accessible once Finalize is called.  We
	// can't support access after Finalize since spockSecretHasher.SumHash is
	// not idempotent.  Repeated calls to SumHash (without modifying the input)
	// may return different hashes.
	finalizedSpockSecret []byte
}

func newSpockState(base snapshot.StorageSnapshot) *spockState {
	return &spockState{
		storageState:      newStorageState(base),
		spockSecretHasher: hash.NewSHA3_256(),
	}
}

func (state *spockState) NewChild() *spockState {
	return &spockState{
		storageState:      state.storageState.NewChild(),
		spockSecretHasher: hash.NewSHA3_256(),
	}
}

func (state *spockState) Finalize() *snapshot.ExecutionSnapshot {
	if state.finalizedSpockSecret == nil {
		state.finalizedSpockSecret = state.spockSecretHasher.SumHash()
	}

	snapshot := state.storageState.Finalize()
	snapshot.SpockSecret = state.finalizedSpockSecret
	return snapshot
}

func (state *spockState) Merge(snapshot *snapshot.ExecutionSnapshot) error {
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

func (state *spockState) Set(
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

func (state *spockState) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	if state.finalizedSpockSecret != nil {
		return nil, fmt.Errorf("cannot Get on a finalized state")
	}

	_, err := state.spockSecretHasher.Write(getMarker)
	if err != nil {
		return nil, fmt.Errorf("get SPoCK failed: %w", err)
	}

	idBytes := id.Bytes()

	// Note: encoding the register id length as part of spock hash to prevent
	// string injection attacks.
	err = binary.Write(
		state.spockSecretHasher,
		binary.LittleEndian,
		int32(len(idBytes)))
	if err != nil {
		return nil, fmt.Errorf("get SPoCK failed: %w", err)
	}

	_, err = state.spockSecretHasher.Write(idBytes)
	if err != nil {
		return nil, fmt.Errorf("get SPoCK failed: %w", err)
	}

	return state.storageState.Get(id)
}

func (state *spockState) DropChanges() error {
	if state.finalizedSpockSecret != nil {
		return fmt.Errorf("cannot DropChanges on a finalized state")
	}

	_, err := state.spockSecretHasher.Write(dropChangesMarker)
	if err != nil {
		return fmt.Errorf("drop changes SPoCK failed: %w", err)
	}

	return state.storageState.DropChanges()
}

func (state *spockState) readSetSize() int {
	return state.storageState.readSetSize()
}

func (state *spockState) interimReadSet(
	accumulator map[flow.RegisterID]struct{},
) {
	state.storageState.interimReadSet(accumulator)
}
