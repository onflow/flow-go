package signature

import (
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
)

// BlockToBytesForSign generates the bytes that was signed for a block
// Note: this function should be reused when signing a block or a vote
func BlockToBytesForSign(block *model.Block) []byte {
	// TODO: we are supposed to sign on `encode(BlockID, View)`
	// but what actually signing is `hash(encoding(BlockID, View))`
	// it works, but the hash is useless, because the signing function
	// in crypto library will always hash it.
	// so instead of using MakeID, we could just return the encoded tuple
	// of BlockID and View
	msg := flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: block.BlockID,
		View:    block.View,
	})
	return msg[:]
}
