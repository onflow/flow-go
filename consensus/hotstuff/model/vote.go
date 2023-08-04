package model

import (
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// Vote is the HotStuff algorithm's concept of a vote for a block proposal.
type Vote struct {
	View     uint64
	BlockID  flow.Identifier
	SignerID flow.Identifier
	SigData  []byte
}

// ID returns the identifier for the vote.
func (uv *Vote) ID() flow.Identifier {
	return flow.MakeID(uv)
}

// VoteFromFlow turns the vote parameters into a vote struct.
func VoteFromFlow(signerID flow.Identifier, blockID flow.Identifier, view uint64, sig crypto.Signature) *Vote {
	vote := Vote{
		View:     view,
		BlockID:  blockID,
		SignerID: signerID,
		SigData:  sig,
	}
	return &vote
}
