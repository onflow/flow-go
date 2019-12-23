package blockRequester

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"

// BlockRequester
type BlockRequester struct {
}

// OnBlockRequest requests a block
func (br *BlockRequester) OnMissingBlock(blockMRH []byte, blockView uint64) {
	panic("Implement me")
}

// OnBlockIncorporated marks `block` as no longer outstanding
// in case the BlockRequester was previously requesting it
func (br *BlockRequester) OnBlockIncorporated(block *def.Block) {
	panic("Implement me")
}
