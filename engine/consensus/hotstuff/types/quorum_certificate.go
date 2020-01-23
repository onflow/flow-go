package types

import "github.com/dapperlabs/flow-go/model/flow"

type QuorumCertificate struct {
	View                uint64
	BlockMRH            flow.Identifier
	AggregatedSignature *AggregatedSignature
}

func (qc QuorumCertificate) BytesForSig() []byte {
	return voteBytesForSig(qc.View, qc.BlockMRH)
}
