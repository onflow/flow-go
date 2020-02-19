package messages

import (
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type BlockProposal struct {
	ChainID       string
	ParentID      flow.Identifier
	View          uint64
	Timestamp     time.Time
	ParentSigs    []crypto.Signature
	ParentSigners []flow.Identifier
	ProposerSig   crypto.Signature
	Payload       *flow.Payload
}

type BlockVote struct {
	BlockID   flow.Identifier
	View      uint64
	Signature crypto.Signature
}

type BlockRequest struct {
	BlockID flow.Identifier
	Nonce   uint64
}

type BlockResponse struct {
	OriginID flow.Identifier
	Proposal *BlockProposal
	Nonce    uint64
}
