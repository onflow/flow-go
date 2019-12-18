package blockRequester

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// BlockRequester
type BlockRequester struct {
}

// OnBlockRequest requests a block
func (br *BlockRequester) OnMissingBlock(blockMRH []byte, blockView uint64) {
	panic("Implement me")
}

// OnIncorporatedBlock marks `block` as no longer outstanding
// in case the BlockRequester was previously requesting it
func (br *BlockRequester) OnIncorporatedBlock(block *def.Block) {
	panic("Implement me")
}
