package payload

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/cargo/storage"
	"github.com/onflow/flow-go/model/flow"
)

// PayloadStore is a fork-aware payload storage
// block updates are accepted through update as long as the parent block results
// has been provided in the past.
// payload store holds forks of updates into memory until a block finalized signal
// is receieved, then it would move finalized results into a persistant storage
// and would prune results for the same height
type PayloadStore struct {
	oracleView       *OracleView
	viewByID         map[flow.Identifier]*InFlightView
	blockIDsByHeight map[uint64]map[flow.Identifier]struct{}
	childrenByID     map[flow.Identifier][]flow.Identifier
	lock             sync.RWMutex
}

func NewPayloadStore(storage storage.Storage) (*PayloadStore, error) {
	oracle, err := NewOracleView(storage)
	if err != nil {
		return nil, err
	}
	return &PayloadStore{
		oracleView:       oracle,
		viewByID:         make(map[flow.Identifier]*InFlightView, 0),
		blockIDsByHeight: make(map[uint64]map[flow.Identifier]struct{}, 0),
		childrenByID:     make(map[flow.Identifier][]flow.Identifier, 0),
		lock:             sync.RWMutex{},
	}, nil
}

func (ps *PayloadStore) Get(
	height uint64,
	blockID flow.Identifier,
	key flow.RegisterID,
) (flow.RegisterValue, error) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	if height <= ps.oracleView.lastCommittedHeight {
		return ps.oracleView.Get(height, blockID, key)
	}

	view, found := ps.viewByID[blockID]
	if !found {
		return nil, fmt.Errorf("view for the block %x is not available", blockID)
	}
	return view.Get(height, blockID, key)
}

func (ps *PayloadStore) Update(
	header *flow.Header,
	delta map[flow.RegisterID]flow.RegisterValue,
) error {
	height := header.Height
	blockID := header.ID()
	parentBlockID := header.ParentID

	ps.lock.Lock()
	defer ps.lock.Unlock()

	// discard
	if height < ps.oracleView.lastCommittedHeight {
		return nil
	}

	var parent View
	parent = ps.oracleView
	if height > ps.oracleView.lastCommittedHeight+1 {
		var found bool
		parent, found = ps.viewByID[parentBlockID]
		if !found {
			// this should never happen, this means updates for a block was submitted but parent is not available
			return fmt.Errorf("view for parent block %x is missing", parentBlockID)
		}
		// keep parent relationship of blocks
		ps.childrenByID[parentBlockID] = append(ps.childrenByID[parentBlockID], blockID)
	}

	// add to view by ID
	ps.viewByID[blockID] = &InFlightView{
		delta:  delta,
		parent: parent,
	}

	// add to the by height index
	dict, found := ps.blockIDsByHeight[height]
	if !found {
		dict = make(map[flow.Identifier]struct{})
	}
	dict[blockID] = struct{}{}
	ps.blockIDsByHeight[height] = dict
	return nil
}

func (ps *PayloadStore) Commit(
	blockID flow.Identifier,
	header *flow.Header,
) (bool, error) {
	height := header.Height

	ps.lock.Lock()
	defer ps.lock.Unlock()

	view, found := ps.viewByID[blockID]
	// return, probably too early
	if !found {
		return false, nil
	}

	// update oracle with the correct one
	err := ps.oracleView.MergeView(header, view)
	if err != nil {
		return false, err
	}

	// remove all views in the same height
	for bID := range ps.blockIDsByHeight[height] {
		delete(ps.viewByID, bID)
		if bID != blockID {
			ps.pruneBranch(bID)
		}

	}
	delete(ps.blockIDsByHeight, height)

	// update all children to use oracle
	for _, childID := range ps.childrenByID[blockID] {
		ps.viewByID[childID].UpdateParent(ps.oracleView)
	}
	delete(ps.childrenByID, blockID)

	return true, nil
}

func (ps *PayloadStore) pruneBranch(blockID flow.Identifier) error {
	for _, child := range ps.childrenByID[blockID] {
		ps.pruneBranch(child)
		delete(ps.viewByID, child)
	}
	delete(ps.childrenByID, blockID)
	return nil
}
