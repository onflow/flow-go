package model

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// Vote is the HotStuff algorithm's concept of a vote for a block proposal.
//
//structwrite:immutable - mutations allowed only within the constructor
type Vote struct {
	View     uint64
	BlockID  flow.Identifier
	SignerID flow.Identifier
	SigData  []byte
}

func NewVote(view uint64, blockID flow.Identifier, signerID flow.Identifier, sigData []byte) Vote {
	return Vote{
		View:     view,
		BlockID:  blockID,
		SignerID: signerID,
		SigData:  sigData,
	}
}

// ID returns the identifier for the vote.
func (uv *Vote) ID() flow.Identifier {
	return flow.MakeID(uv)
}

// VoteFromFlow turns the vote parameters into a vote struct.
func VoteFromFlow(signerID flow.Identifier, blockID flow.Identifier, view uint64, sig crypto.Signature) *Vote {
	vote := NewVote(
		view,
		blockID,
		signerID,
		sig,
	)

	return &vote
}
