package types

import (
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/crypto"
)

type QuorumCertificate struct {
	View                uint64
	BlockMRH            []byte
	AggregatedSignature *AggregatedSignature
}

func NewQC(block *Block, sigs []*Signature, signersBitfieldLength uint32) *QuorumCertificate {
	qc := &QuorumCertificate{
		BlockMRH:            block.BlockMRH(),
		View:                block.View,
		AggregatedSignature: crypto.AggregateSigs(sigs, signersBitfieldLength),
	}

	return qc
}
