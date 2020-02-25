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
	BlockID               flow.Identifier
	View                  uint64
	StakingSignature      crypto.Signature
	RandomBeaconSignature crypto.Signature
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
