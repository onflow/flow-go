package mocks

import (
	"fmt"
	"sync"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
)

// ProtocolState is a mocked version of protocol state, which
// has very close behavior to the real implementation
// but for testing purpose.
// If you are testing a module that depends on protocol state's
// behavior, but you don't want to mock up the methods and its return
// value, then just use this module
type ProtocolState struct {
	sync.Mutex
	protocol.MutableState
	blocks    map[flow.Identifier]*flow.Block
	children  map[flow.Identifier][]flow.Identifier
	heights   map[uint64]*flow.Block
	finalized uint64
	root      *flow.Block
	result    *flow.ExecutionResult
	seal      *flow.Seal
}

func NewProtocolState() *ProtocolState {
	return &ProtocolState{
		blocks:   make(map[flow.Identifier]*flow.Block),
		children: make(map[flow.Identifier][]flow.Identifier),
		heights:  make(map[uint64]*flow.Block),
	}
}

type Params struct {
	state *ProtocolState
}

func (p *Params) ChainID() (flow.ChainID, error) {
	return p.state.root.Header.ChainID, nil
}

func (p *Params) SporkID() (flow.Identifier, error) {
	return flow.ZeroID, fmt.Errorf("not implemented")
}

func (p *Params) ProtocolVersion() (uint, error) {
	return 0, fmt.Errorf("not implemented")
}

func (p *Params) Root() (*flow.Header, error) {
	return p.state.root.Header, nil
}

func (p *Params) Seal() (*flow.Seal, error) {
	return nil, fmt.Errorf("not implemented")
}

func (ps *ProtocolState) Params() protocol.Params {
	return &Params{
		state: ps,
	}
}

func (ps *ProtocolState) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	ps.Lock()
	defer ps.Unlock()

	snapshot := new(protocolmock.Snapshot)
	block, ok := ps.blocks[blockID]
	if ok {
		snapshot.On("Head").Return(block.Header, nil)
	} else {
		snapshot.On("Head").Return(nil, storage.ErrNotFound)
	}
	return snapshot
}

func (ps *ProtocolState) AtHeight(height uint64) protocol.Snapshot {
	ps.Lock()
	defer ps.Unlock()

	snapshot := new(protocolmock.Snapshot)
	block, ok := ps.heights[height]
	if ok {
		snapshot.On("Head").Return(block.Header, nil)
	} else {
		snapshot.On("Head").Return(nil, storage.ErrNotFound)
	}
	return snapshot
}

func (ps *ProtocolState) Final() protocol.Snapshot {
	ps.Lock()
	defer ps.Unlock()

	final, ok := ps.heights[ps.finalized]
	if !ok {
		return nil
	}

	snapshot := new(protocolmock.Snapshot)
	snapshot.On("Head").Return(final.Header, nil)
	finalID := final.ID()
	mocked := snapshot.On("Descendants")
	mocked.RunFn = func(args mock.Arguments) {
		// not concurrent safe
		pendings := pending(ps, finalID)
		mocked.ReturnArguments = mock.Arguments{pendings, nil}
	}

	mocked = snapshot.On("ValidDescendants")
	mocked.RunFn = func(args mock.Arguments) {
		// not concurrent safe
		pendings := pending(ps, finalID)
		mocked.ReturnArguments = mock.Arguments{pendings, nil}
	}
	return snapshot
}

func pending(ps *ProtocolState, blockID flow.Identifier) []flow.Identifier {
	var pendingIDs []flow.Identifier
	pendingIDs, ok := ps.children[blockID]

	if !ok {
		return pendingIDs
	}

	for _, pendingID := range pendingIDs {
		additionalIDs := pending(ps, pendingID)
		pendingIDs = append(pendingIDs, additionalIDs...)
	}

	return pendingIDs
}

func (m *ProtocolState) Bootstrap(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.blocks[root.ID()]; ok {
		return storage.ErrAlreadyExists
	}

	m.blocks[root.ID()] = root
	m.root = root
	m.result = result
	m.seal = seal
	m.heights[root.Header.Height] = root
	m.finalized = root.Header.Height
	return nil
}

func (m *ProtocolState) Extend(block *flow.Block) error {
	m.Lock()
	defer m.Unlock()

	id := block.ID()
	if _, ok := m.blocks[id]; ok {
		return storage.ErrAlreadyExists
	}

	if _, ok := m.blocks[block.Header.ParentID]; !ok {
		return fmt.Errorf("could not retrieve parent")
	}

	m.blocks[id] = block

	// index children
	children, ok := m.children[block.Header.ParentID]
	if !ok {
		children = make([]flow.Identifier, 0)
	}

	children = append(children, id)
	m.children[block.Header.ParentID] = children

	return nil
}

func (m *ProtocolState) Finalize(blockID flow.Identifier) error {
	m.Lock()
	defer m.Unlock()

	block, ok := m.blocks[blockID]
	if !ok {
		return fmt.Errorf("could not retrieve final header")
	}

	if block.Header.Height <= m.finalized {
		return fmt.Errorf("could not finalize old blocks")
	}

	// update heights
	cur := block
	for height := cur.Header.Height; height > m.finalized; height-- {
		parent, ok := m.blocks[cur.Header.ParentID]
		if !ok {
			return fmt.Errorf("parent does not exist for block at height: %v, parentID: %v", cur.Header.Height, cur.Header.ParentID)
		}
		m.heights[height] = cur
		cur = parent
	}

	m.finalized = block.Header.Height

	return nil
}
