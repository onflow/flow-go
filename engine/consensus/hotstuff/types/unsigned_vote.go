package types

import "github.com/dapperlabs/flow-go/model/flow"

type UnsignedVote struct {
	View     uint64
	BlockMRH flow.Identifier
}

func NewUnsignedVote(view uint64, blockMRH flow.Identifier) *UnsignedVote {
	return &UnsignedVote{
		View:     view,
		BlockMRH: blockMRH,
	}
}

func (uv UnsignedVote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockMRH)
}
