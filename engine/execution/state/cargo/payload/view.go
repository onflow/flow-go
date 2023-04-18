package payload

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type View interface {
	Get(height uint64, blockID flow.Identifier, key flow.RegisterID) (flow.RegisterValue, error)
}

type OracleView struct {
	storage Storage
	// cached value of the same data that is retrivable from storage
	lastCommittedHeight uint64
}

func NewOracleView(storage Storage) (*OracleView, error) {
	_, header, err := storage.LastCommittedBlock()
	if err != nil {
		return nil, err
	}
	return &OracleView{
		storage:             storage,
		lastCommittedHeight: header.Height,
	}, nil
}

var _ View = &OracleView{}

func (v *OracleView) Get(height uint64, _ flow.Identifier, id flow.RegisterID) (flow.RegisterValue, error) {
	// TODO: id could be used as extra sanity check
	return v.storage.RegisterValueAt(height, id)
}

func (v *OracleView) MergeView(header *flow.Header, view *InFlightView) error {
	err := v.storage.Commit(header, view.delta)
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

func (v *InFlightView) Get(height uint64, blockID flow.Identifier, key flow.RegisterID) (flow.RegisterValue, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()

	value, found := v.delta[key]
	if found {
		return value, nil
	}

	return v.parent.Get(height, blockID, key)
}

func (v *InFlightView) UpdateParent(new View) {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.parent = new
}
