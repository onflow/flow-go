package delta

// TODO(patrick): rm after updating emulator

import (
	"github.com/onflow/flow-go/fvm/storage/state"
)

func NewDeltaView(storage state.StorageSnapshot) state.View {
	return state.NewExecutionState(
		storage,
		state.DefaultParameters())
}
