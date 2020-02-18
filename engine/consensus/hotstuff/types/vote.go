package types

import (
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
