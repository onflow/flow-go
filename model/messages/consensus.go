package messages

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type BlockProposal struct {
	Header  *flow.Header
	Payload *flow.Payload
}

type BlockVote struct {
	BlockID   flow.Identifier
	View      uint64
	Signature crypto.Signature
}
