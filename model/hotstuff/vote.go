package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Vote struct {
	BlockID   flow.Identifier
	View      uint64
	Signature *SingleSignature
}

func (uv *Vote) ID() flow.Identifier {
	return flow.MakeID(uv)
}

// VoteFromFlow turns the vote parameters into a vote struct.
func VoteFromFlow(signerID flow.Identifier, blockID flow.Identifier, view uint64, raw crypto.Signature) *Vote {
	sig := SingleSignature{
		Raw:      raw,
		SignerID: signerID,
	}
	vote := Vote{
		BlockID:   blockID,
		View:      view,
		Signature: &sig,
	}
	return &vote
}
