package forest

import (
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
	storage            storage.Storage
	viewByID           map[flow.Identifier]*InFlightView
	blockIDsByHeight   map[uint64]map[flow.Identifier]struct{}
	childrenByID       map[flow.Identifier][]flow.Identifier
	lastCommittedBlock *flow.Header // cached value for the last committed header
	lock               sync.RWMutex
}

var _ storage.ForkAwareStorage = &ForestStorage{}

// NewStorage constructs a new ForestStorage
func NewStorage(storage storage.Storage) (*ForestStorage, error) {
	header, err := storage.LastCommittedBlock()
	if err != nil {
		return nil, err
	}
	return &ForestStorage{
		storage:            storage,
		viewByID:           make(map[flow.Identifier]*InFlightView, 0),
		blockIDsByHeight:   make(map[uint64]map[flow.Identifier]struct{}, 0),
		childrenByID:       make(map[flow.Identifier][]flow.Identifier, 0),
		lock:               sync.RWMutex{},
		lastCommittedBlock: header,
	}, nil
}

func (fs *ForestStorage) BlockView(
	height uint64,
	blockID flow.Identifier,
) (storage.BlockView, error) {
	fs.lock.RLock()
	defer fs.lock.RUnlock()

	if height <= fs.lastCommittedBlock.Height {
		return fs.storage.BlockView(height, blockID)
	}

	view, found := fs.viewByID[blockID]
	if !found {
		return nil, &storage.InvalidBlockIDError{BlockID: blockID}
	}
	return view, nil
}

func (fs *ForestStorage) CommitBlock(
	header *flow.Header,
	delta map[flow.RegisterID]flow.RegisterValue,
) error {
	height := header.Height
	blockID := header.ID()
	parentBlockID := header.ParentID

	fs.lock.Lock()
	defer fs.lock.Unlock()

	// discard (TODO: maybe return error in the future)
	if height <= fs.lastCommittedBlock.Height {
		return nil
	}

	var parent storage.BlockView
	if height-1 <= fs.lastCommittedBlock.Height {
		var err error
		parent, err = fs.storage.BlockView(height-1, parentBlockID)
		if err != nil {
			return &storage.ParentBlockNotFoundError{ParentBlockID: parentBlockID}
		}
	} else {
		var found bool
		parent, found = fs.viewByID[parentBlockID]
		if !found {
			// this should never happen, this means updates for a block was submitted but parent is not available
			return &storage.ParentBlockNotFoundError{ParentBlockID: parentBlockID}
		}
	}

	// update children list
	fs.childrenByID[parentBlockID] = append(fs.childrenByID[parentBlockID], blockID)

	// add to view by ID
	fs.viewByID[blockID] = &InFlightView{
		delta:  delta,
		parent: parent,
	}

	// add to the by height index
	dict, found := fs.blockIDsByHeight[height]
	if !found {
		dict = make(map[flow.Identifier]struct{})
	}
	dict[blockID] = struct{}{}
	fs.blockIDsByHeight[height] = dict
	return nil
}

func (fs *ForestStorage) LastCommittedBlock() (*flow.Header, error) {
	return fs.lastCommittedBlock, nil
}

func (fs *ForestStorage) BlockFinalized(header *flow.Header) (bool, error) {
	height := header.Height
	blockID := header.ID()

	fs.lock.Lock()
	defer fs.lock.Unlock()

	view, found := fs.viewByID[blockID]
	// return, probably too early,
	// return with false flag so we don't consume the event
	if !found {
		return false, nil
	}

	// update oracle with the correct one
	err := fs.storage.CommitBlock(header, view.delta)
	if err != nil {
		return false, err
	}

	// find all blocks with the given height
	// remove them from the viewByID list
	// if not our target block, prune children
	for bID := range fs.blockIDsByHeight[height] {
		delete(fs.viewByID, bID)
		if bID != blockID {
			fs.pruneBranch(bID)
		}

	}
	// remove this height
	delete(fs.blockIDsByHeight, height)

	// update all child blocks to use the oracle view
	for _, childID := range fs.childrenByID[blockID] {
		bv, err := fs.storage.BlockView(height, blockID)
		if err != nil {
			return false, err
		}
		fs.viewByID[childID].UpdateParent(bv)
	}
	// remove childrenByID lookup
	delete(fs.childrenByID, blockID)

	fs.lastCommittedBlock = header

	return true, nil
}

func (fs *ForestStorage) pruneBranch(blockID flow.Identifier) error {
	for _, child := range fs.childrenByID[blockID] {
		fs.pruneBranch(child)
		delete(fs.viewByID, child)
	}
	delete(fs.childrenByID, blockID)
	return nil
}
