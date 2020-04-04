package metrics

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type VerificationMetricConsumer interface {
	OnChunkVerificationStated(chunkID flow.Identifier)
	OnChunkVerificationFinished(chunkID flow.Identifier, blockID flow.Identifier)
	OnResultApproval(blockID flow.Identifier)
}
