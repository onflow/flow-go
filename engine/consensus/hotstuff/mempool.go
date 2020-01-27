package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Mempool interface {
	NewPayloadHash() []byte
	NewConsensusPayload() *types.ConsensusPayload
}
