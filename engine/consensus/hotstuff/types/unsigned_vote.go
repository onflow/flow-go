package types

import "github.com/dapperlabs/flow-go/model/flow"

type UnsignedVote struct {
	View    uint64
	BlockID flow.Identifier
}

func NewUnsignedVote(view uint64, blockID flow.Identifier) *UnsignedVote {
	return &UnsignedVote{
		View:    view,
		BlockID: blockID,
	}
}

func (uv *UnsignedVote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockID)
}

func (uv UnsignedVote) WithSignature(sig *Signature) *Vote {
	return &Vote{
		View:      uv.View,
		BlockID:   uv.BlockID,
		Signature: sig,
	}
}
