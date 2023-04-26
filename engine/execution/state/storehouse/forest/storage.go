package forest

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage"
	"github.com/onflow/flow-go/model/flow"
)

// ForestStorage is a fork-aware storage
// acts as a wrapper on any non-forkaware storage implementation
// block updates are accepted as long as the parent block update
// has been provided in the past.
// it holds forks of updates into memory until a block finalized signal
// is receieved, then it would move finalized results into a persistant storage
// and would prune and discard block changes that are not relevant anymore.
type ForestStorage struct {
	storage             storage.Storage
	viewByID            map[flow.Identifier]*InFlightView
	blockIDsByHeight    map[uint64]map[flow.Identifier]struct{}
	childrenByID        map[flow.Identifier][]flow.Identifier
	lastCommittedHeight uint64 // a cached value for the last committed height
	lock                sync.RWMutex
}

// NewStorage constructs a new ForestStorage
func NewStorage(storage storage.Storage) (*ForestStorage, error) {
	header, err := storage.LastCommittedBlock()
	if err != nil {
		return nil, err
	}
	return &ForestStorage{
		storage:             storage,
		viewByID:            make(map[flow.Identifier]*InFlightView, 0),
		blockIDsByHeight:    make(map[uint64]map[flow.Identifier]struct{}, 0),
		childrenByID:        make(map[flow.Identifier][]flow.Identifier, 0),
		lock:                sync.RWMutex{},
		lastCommittedHeight: header.Height,
	}, nil
}

func (ps *ForestStorage) BlockView(
	height uint64,
	blockID flow.Identifier,
) (storage.BlockView, error) {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	if height <= ps.lastCommittedHeight {
		return ps.storage.BlockView(height, blockID)
	}

	view, found := ps.viewByID[blockID]
	if !found {
		return nil, fmt.Errorf("view for the block %x is not available", blockID)
	}
	return view, nil
}

func (ps *ForestStorage) BlockExecuted(
	header *flow.Header,
	delta map[flow.RegisterID]flow.RegisterValue,
) error {
	height := header.Height
	blockID := header.ID()
	parentBlockID := header.ParentID

	ps.lock.Lock()
	defer ps.lock.Unlock()

	// discard (TODO: maybe return error in the future)
	if height <= ps.lastCommittedHeight {
		return nil
	}

	var parent storage.BlockView
	if height-1 <= ps.lastCommittedHeight {
		var err error
		parent, err = ps.storage.BlockView(height-1, parentBlockID)
		if err != nil {
			return fmt.Errorf("parent block view for %x is missing (storage)", parentBlockID)
		}
	} else {
		var found bool
		parent, found = ps.viewByID[parentBlockID]
		if !found {
			// this should never happen, this means updates for a block was submitted but parent is not available
			return fmt.Errorf("block view for %x is missing (in flight)", parentBlockID)
		}
	}

	// update children list
	ps.childrenByID[parentBlockID] = append(ps.childrenByID[parentBlockID], blockID)

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

func (ps *ForestStorage) BlockFinalized(
	blockID flow.Identifier,
	header *flow.Header,
) (bool, error) {
	height := header.Height

	ps.lock.Lock()
	defer ps.lock.Unlock()

	view, found := ps.viewByID[blockID]
	// return, probably too early,
	// return with false flag so we don't consume the event
	if !found {
		return false, nil
	}

	// update oracle with the correct one
	err := ps.storage.CommitBlock(header, view.delta)
	if err != nil {
		return false, err
	}

	// find all blocks with the given height
	// remove them from the viewByID list
	// if not our target block, prune children
	for bID := range ps.blockIDsByHeight[height] {
		delete(ps.viewByID, bID)
		if bID != blockID {
			ps.pruneBranch(bID)
		}

	}
	// remove this height
	delete(ps.blockIDsByHeight, height)

	// update all child blocks to use the oracle view
	for _, childID := range ps.childrenByID[blockID] {
		bv, err := ps.storage.BlockView(height, blockID)
		if err != nil {
			return false, err
		}
		ps.viewByID[childID].UpdateParent(bv)
	}
	// remove childrenByID lookup
	delete(ps.childrenByID, blockID)

	ps.lastCommittedHeight = header.Height

	return true, nil
}

func (ps *ForestStorage) pruneBranch(blockID flow.Identifier) error {
	for _, child := range ps.childrenByID[blockID] {
		ps.pruneBranch(child)
		delete(ps.viewByID, child)
	}
	delete(ps.childrenByID, blockID)
	return nil
}
