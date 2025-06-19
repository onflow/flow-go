package model

import (
	"fmt"

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

// UntrustedVote is an untrusted input-only representation of an Vote,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedVote should be validated and converted into
// a trusted Vote using NewVote constructor.
type UntrustedVote Vote

// NewVote creates a new instance of Vote.
// Construction Vote allowed only within the constructor
//
// All errors indicate a valid Vote cannot be constructed from the input.
func NewVote(untrusted UntrustedVote) (*Vote, error) {
	if untrusted.BlockID == flow.ZeroID {
		return nil, fmt.Errorf("BlockID must not be empty")
	}

	if untrusted.SignerID == flow.ZeroID {
		return nil, fmt.Errorf("SignerID must not be empty")
	}

	if len(untrusted.SigData) == 0 {
		return nil, fmt.Errorf("SigData must not be empty")
	}

	return &Vote{
		View:     untrusted.View,
		BlockID:  untrusted.BlockID,
		SignerID: untrusted.SignerID,
		SigData:  untrusted.SigData,
	}, nil
}

// ID returns the identifier for the vote.
func (uv *Vote) ID() flow.Identifier {
	return flow.MakeID(uv)
}
