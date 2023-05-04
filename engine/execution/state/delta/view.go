package delta

// TODO(patrick): rm after updating emulator

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

func NewDeltaView(storage snapshot.StorageSnapshot) state.View {
	return state.NewExecutionState(
		storage,
		state.DefaultParameters())
}
