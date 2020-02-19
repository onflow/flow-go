package hotstuff

import "github.com/dapperlabs/flow-go/model/flow"

type QuorumCertificate struct {
	View                uint64
	BlockID             flow.Identifier
	AggregatedSignature *AggregatedSignature
}
