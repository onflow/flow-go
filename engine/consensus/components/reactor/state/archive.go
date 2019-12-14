package state

import "github.com/dapperlabs/flow-go/engine/consensus/modules/def"

type Archive interface {
	Store(block *def.Block)
	RetrieveByHash(hash []byte) (*def.Block, bool)
	RetrieveByView(view uint64) (*def.Block, bool)
}
