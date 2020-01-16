package types

type QuorumCertificate struct {
	View                uint64
	BlockMRH            []byte
	AggregatedSignature *AggregatedSignature
}

func NewQC(block *Block, sigs []byte, signersBitfieldLength []bool) *QuorumCertificate {
	qc := &QuorumCertificate{
		BlockMRH: block.BlockMRH(),
		View:     block.View,
		AggregatedSignature: &AggregatedSignature{
			RawSignature: sigs,
			Signers:      signersBitfieldLength,
		},
	}

	return qc
}
