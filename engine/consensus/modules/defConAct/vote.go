package defConAct

import "github.com/dapperlabs/flow-go/engine/consensus/modules/crypto"

type Vote struct {
	// TODO: Why is view here?
	View             uint64
	BlockMRH         []byte
	ContentSignature *crypto.Signature
}

func NewVote(view uint64, blockMRH []byte, contentSignature *crypto.Signature) *Vote {
	return &Vote{
		View:             view,
		BlockMRH:         blockMRH,
		ContentSignature: contentSignature,
	}
}
