package state

import "github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"

type memArchive struct {
	blocksByHash map[string]*def.Block
	blocksByView map[uint64]*def.Block
}

func (m *memArchive) Store(block *def.Block) {
	b, exists := m.blocksByHash[string(block.BlockMRH)]
	if exists {
		if block.View != b.View {
			panic("Encountered two blocks with same hash but different View Number")
		}
		return
	}
	m.blocksByHash[string(block.BlockMRH)] = block
}

func (m *memArchive) RetrieveByHash(hash []byte) (*def.Block, bool) {
	b, exists := m.blocksByHash[string(hash)]
	if !exists {
		return nil, false
	}
	return b, true
}

func (m *memArchive) RetrieveByView(view uint64) (*def.Block, bool) {
	b, exists := m.blocksByView[view]
	if !exists {
		return nil, false
	}
	return b, true
}
