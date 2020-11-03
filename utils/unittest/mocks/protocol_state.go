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
	protocol.State
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

type ProtocolStateMutator struct {
	protocolmock.Mutator
	ps *ProtocolState
}

type Params struct {
	state *ProtocolState
}

func (p *Params) ChainID() (flow.ChainID, error) {
	return p.state.root.Header.ChainID, nil
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
	mocked := snapshot.On("Pending")
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

func (ps *ProtocolState) Mutate() protocol.Mutator {
	return &ProtocolStateMutator{
		protocolmock.Mutator{},
		ps,
	}
}

func (m *ProtocolStateMutator) Bootstrap(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error {
	m.ps.Lock()
	defer m.ps.Unlock()

	if _, ok := m.ps.blocks[root.ID()]; ok {
		return storage.ErrAlreadyExists
	}

	m.ps.blocks[root.ID()] = root
	m.ps.root = root
	m.ps.result = result
	m.ps.seal = seal
	m.ps.heights[root.Header.Height] = root
	m.ps.finalized = root.Header.Height
	return nil
}

func (m *ProtocolStateMutator) Extend(block *flow.Block) error {
	m.ps.Lock()
	defer m.ps.Unlock()

	id := block.ID()
	if _, ok := m.ps.blocks[id]; ok {
		return storage.ErrAlreadyExists
	}

	if _, ok := m.ps.blocks[block.Header.ParentID]; !ok {
		return fmt.Errorf("could not retrieve parent")
	}

	m.ps.blocks[id] = block

	// index children
	children, ok := m.ps.children[block.Header.ParentID]
	if !ok {
		children = make([]flow.Identifier, 0)
	}

	children = append(children, id)
	m.ps.children[block.Header.ParentID] = children

	return nil
}

func (m *ProtocolStateMutator) Finalize(blockID flow.Identifier) error {
	m.ps.Lock()
	defer m.ps.Unlock()

	block, ok := m.ps.blocks[blockID]
	if !ok {
		return fmt.Errorf("could not retrieve final header")
	}

	if block.Header.Height <= m.ps.finalized {
		return fmt.Errorf("could not finalize old blocks")
	}

	// update heights
	cur := block
	for height := cur.Header.Height; height > m.ps.finalized; height-- {
		parent, ok := m.ps.blocks[cur.Header.ParentID]
		if !ok {
			return fmt.Errorf("parent does not exist for block at height: %v, parentID: %v", cur.Header.Height, cur.Header.ParentID)
		}
		m.ps.heights[height] = cur
		cur = parent
	}

	m.ps.finalized = block.Header.Height

	return nil
}
