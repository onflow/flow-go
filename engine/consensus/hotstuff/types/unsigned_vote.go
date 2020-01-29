package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

type UnsignedVote struct {
	View    uint64
	BlockID flow.Identifier
}

func NewUnsignedVote(view uint64, blockMRH flow.Identifier) *UnsignedVote {
	return &UnsignedVote{
		View:    view,
		BlockID: blockMRH,
	}
}

func (uv UnsignedVote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockID)
}

func voteBytesForSig(view uint64, blockID flow.Identifier) []byte {
	prefix := []byte("vote")
	bytes := marshalForSig(NewUnsignedVote(view, blockID))
	return append(prefix, bytes...)
}

func marshalForSig(v *UnsignedVote) []byte {
	data := encoding.DefaultEncoder.MustEncode(v)
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	hash := hasher.ComputeHash(data)
	return hash
}
