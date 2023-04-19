package payload

import (
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	Get(
		height uint64,
		blockID flow.Identifier,
		key flow.RegisterID,
	) (
		flow.RegisterValue,
		error,
	)
}

// oracle stores views that are finalized in an storage
type OracleView struct {
	storage storage.Storage

	// cached value of the same data that is retrivable from storage
	lastCommittedHeight uint64
}

func NewOracleView(storage storage.Storage) (*OracleView, error) {
	header, err := storage.LastCommittedBlock()
	if err != nil {
		return nil, err
	}
	return &OracleView{
		storage:             storage,
		lastCommittedHeight: header.Height,
	}, nil
}

var _ View = &OracleView{}

func (v *OracleView) Get(height uint64, blockID flow.Identifier, id flow.RegisterID) (flow.RegisterValue, error) {
	return v.storage.RegisterValueAt(height, blockID, id)
}

func (v *OracleView) MergeView(header *flow.Header, view *InFlightView) error {
	err := v.storage.CommitBlock(header, view.delta)
	if err != nil {
		return err
	}
	v.lastCommittedHeight = header.Height
	return nil
}

type InFlightView struct {
	delta  map[flow.RegisterID]flow.RegisterValue
	lock   sync.RWMutex
	parent View
}

var _ View = &InFlightView{}

func NewInFlightView(
	delta map[flow.RegisterID]flow.RegisterValue,
	parent View,
) *InFlightView {
	return &InFlightView{
		delta:  delta,
		parent: parent,
	}
}

// returns the register at the given height
func (v *InFlightView) Get(height uint64, blockID flow.Identifier, key flow.RegisterID) (flow.RegisterValue, error) {
	value, found := v.delta[key]
	if found {
		return value, nil
	}

	v.lock.RLock()
	defer v.lock.RUnlock()

	return v.parent.Get(height, blockID, key)
}

func (v *InFlightView) UpdateParent(new View) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.parent = new
}
