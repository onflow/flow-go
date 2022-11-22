package delta

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// Idea: we update oracle to act based on a min number of blocks in memory (10 for example), then if we have WALs for blockViews
// we could do charge more for access to registers beyond that deltas

type Oracle struct {
	storage Storage

	// TODO use more advanced design instead of a simple mutex (e.g. concurrent seal and newblock calls)
	lock                   sync.Mutex
	blocksInFlightByID     map[flow.Identifier]*BlockView
	blocksInFlightByHeight map[uint64][]flow.Identifier
	childrenViewsByBlockID map[flow.Identifier][]BlockView
}

func NewOracle(storage Storage) (*Oracle, error) {
	return &Oracle{
		storage:                storage,
		lock:                   sync.Mutex{},
		blocksInFlightByID:     make(map[flow.Identifier]*BlockView),
		blocksInFlightByHeight: make(map[uint64][]flow.Identifier),
		childrenViewsByBlockID: make(map[flow.Identifier][]BlockView),
	}, nil
}

func (o *Oracle) recordNewView(view *BlockView, blockID, parentID flow.Identifier, height uint64) {
	o.blocksInFlightByID[blockID] = view

	// setup children for parent block
	childrenViews := o.childrenViewsByBlockID[parentID]
	if len(childrenViews) == 0 {
		o.childrenViewsByBlockID[parentID] = []BlockView{*view}
	} else {
		o.childrenViewsByBlockID[parentID] = append(childrenViews, *view)
	}

	// setup block in flight by height
	blockIDs := o.blocksInFlightByHeight[height]
	if len(blockIDs) == 0 {
		o.blocksInFlightByHeight[height] = []flow.Identifier{blockID}
	} else {
		o.blocksInFlightByHeight[height] = append(blockIDs, blockID)
	}
}

func (o *Oracle) NewBlockView(blockID flow.Identifier, header *flow.Header) (*BlockView, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	// check we don't already have a view with this blockID
	_, found := o.blocksInFlightByID[blockID]
	if found {
		return nil, errors.New("block view already been created for this block in the previous calls")
	}

	parentID := header.ParentID
	height := header.Height

	// handling the case parent is already sealed and/or start of the network
	lsh, err := o.storage.BlockHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to load the latest stored height: %w", err)
	}
	if height <= lsh {
		return nil, fmt.Errorf("requested header has height smaller than last stored height: %d < %d", height, lsh)
	}
	if height == lsh+1 {
		newView := NewBlockView(func(owner, key string) (flow.RegisterValue, error) {
			val, _, err := o.storage.GetRegister(flow.RegisterID{Owner: owner, Key: key})
			// TODO handle exists
			return val, err
		})
		o.recordNewView(newView, blockID, parentID, height)
		return newView, nil
	}

	// find the parent view
	parentView, found := o.blocksInFlightByID[parentID]
	if !found {
		return nil, fmt.Errorf("parent not found (last sealed block: %d, view requested for block height: %d", lsh, header.Height)
	}
	newView := NewBlockView(parentView.Get)
	o.recordNewView(newView, blockID, parentID, height)
	return newView, nil
}

func (o *Oracle) BlockIsSealed(blockID flow.Identifier, header *flow.Header) error {
	o.lock.Lock()
	defer o.lock.Unlock()
	// TODO check header.Height and if its alread merged ignore it (out of order seal calls)

	h, err := o.storage.BlockHeight()
	if err != nil {
		return err
	}
	if header.Height != h+1 {
		return errors.New("block height mismatch (out-of-sync)")
	}

	// find the view for the block
	blockView := o.blocksInFlightByID[blockID]
	// TODO handle the case of not found (alread merged vs not reached there yete)
	if blockView == nil {
		return errors.New("block view not found for the given blockID (out-of-sync)")
	}

	// merge all registers back
	o.storage.CommitBlockDelta(header.Height, blockView.Delta)

	// update all children
	children := o.childrenViewsByBlockID[blockID]
	for _, child := range children {
		child.ReadFunc = func(owner, key string) (flow.RegisterValue, error) {
			val, _, err := o.storage.GetRegister(flow.RegisterID{Owner: owner, Key: key})
			// TODO handle exists
			return val, err
		}
	}

	// clean up branches
	siblings := o.blocksInFlightByHeight[header.Height]
	for _, sibling := range siblings {
		delete(o.blocksInFlightByID, sibling)
	}
	delete(o.blocksInFlightByHeight, header.Height)
	delete(o.childrenViewsByBlockID, blockID)

	return nil
}

func (o *Oracle) LastSealedBlockHeight() (uint64, error) {
	return o.storage.BlockHeight()
}

func (o *Oracle) BlocksInFlight() int {
	return len(o.blocksInFlightByID)
}

func (o *Oracle) HeightsInFlight() int {
	return len(o.blocksInFlightByHeight)
}
