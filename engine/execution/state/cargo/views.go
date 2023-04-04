package cargo

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type Views struct {
	oracleView       *OracleView
	viewByID         map[flow.Identifier]*InFlightView
	blockIDsByHeight map[uint64][]flow.Identifier
	lock             sync.RWMutex
}

func NewViews(storage Storage) (*Views, error) {
	oracle, err := NewOracleView(storage)
	if err != nil {
		return nil, err
	}
	return &Views{
		oracleView:       oracle,
		viewByID:         make(map[flow.Identifier]*InFlightView, 0),
		blockIDsByHeight: make(map[uint64][]flow.Identifier, 0),
		lock:             sync.RWMutex{},
	}, nil
}

func (vs *Views) Get(
	height uint64,
	blockID flow.Identifier,
	key flow.RegisterID,
) (flow.RegisterValue, error) {
	vs.lock.RLock()
	defer vs.lock.RUnlock()

	if height <= vs.oracleView.LastCommittedHeight {
		return vs.oracleView.Get(height, blockID, key)
	}

	view, found := vs.viewByID[blockID]
	if !found {
		return nil, fmt.Errorf("view for the block %x is not available", blockID)
	}
	return view.Get(height, blockID, key)
}

func (vs *Views) Set(
	header *flow.Header,
	delta map[flow.RegisterID]flow.RegisterValue,
) error {
	height := header.Height
	blockID := header.ID()
	parentBlockID := header.ParentID

	vs.lock.Lock()
	defer vs.lock.Unlock()

	// discard
	if height < vs.oracleView.LastCommittedHeight {
		return nil
	}

	var parent View
	parent = vs.oracleView
	if height > vs.oracleView.LastCommittedHeight {
		var found bool
		parent, found = vs.viewByID[parentBlockID]
		if !found {
			// this should never happen, this means updates for a block was submitted but parent is not available
			return fmt.Errorf("view for parent block %x is missing", parentBlockID)
		}
	}

	vs.viewByID[blockID] = &InFlightView{
		delta:  delta,
		parent: parent,
	}

	vs.blockIDsByHeight[height] = append(vs.blockIDsByHeight[height], blockID)
	return nil
}

func (vs *Views) Commit(
	blockID flow.Identifier,
	header *flow.Header,
) (bool, error) {
	height := header.Height

	vs.lock.Lock()
	defer vs.lock.Unlock()

	view, found := vs.viewByID[blockID]
	// return, probably too early
	if !found {
		return false, nil
	}

	// update oracle with the correct one
	err := vs.oracleView.MergeView(header, view)
	if err != nil {
		return false, err
	}

	// remove all views in the same height
	for _, blockID := range vs.blockIDsByHeight[height] {
		delete(vs.viewByID, blockID)
	}
	delete(vs.blockIDsByHeight, height)

	// update all children to use oracle
	for _, blockID := range vs.blockIDsByHeight[height+1] {
		vs.viewByID[blockID].UpdateParent(vs.oracleView)
	}

	return true, nil
}
