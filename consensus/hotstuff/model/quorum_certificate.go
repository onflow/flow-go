package model

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type QuorumCertificate struct {
	View                uint64
	BlockID             flow.Identifier
	AggregatedSignature *AggregatedSignature
	SignerIDs           []flow.Identifier // to be used in the new verification component
	SigData             []byte            // to be used in the new verification compononte
}
