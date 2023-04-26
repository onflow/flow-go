package forest

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage"
	"github.com/onflow/flow-go/model/flow"
)

type InFlightView struct {
	delta  map[flow.RegisterID]flow.RegisterValue
	lock   sync.RWMutex
	parent storage.BlockView
}

var _ storage.BlockView = &InFlightView{}

func NewInFlightView(
	delta map[flow.RegisterID]flow.RegisterValue,
	parent storage.BlockView,
) *InFlightView {
	return &InFlightView{
		delta:  delta,
		parent: parent,
	}
}

// returns the register at the given height
func (v *InFlightView) Get(key flow.RegisterID) (flow.RegisterValue, error) {
	value, found := v.delta[key]
	if found {
		return value, nil
	}

	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.parent.Get(key)
}

func (v *InFlightView) UpdateParent(new storage.BlockView) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.parent = new
}
