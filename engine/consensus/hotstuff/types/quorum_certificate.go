package types

type QuorumCertificate struct {
	View                uint64
	BlockMRH            []byte
	AggregatedSignature *AggregatedSignature
}

func NewQC(block *Block, sigs []*Signature) *QuorumCertificate {
	// TODO: need to add aggregate sig
	qc := &QuorumCertificate{
		BlockMRH: block.BlockMRH(),
		View:     block.View,
	}

	return qc
}
