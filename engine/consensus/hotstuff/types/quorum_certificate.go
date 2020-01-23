package types

type QuorumCertificate struct {
	View                uint64
	BlockMRH            []byte
	AggregatedSignature *AggregatedSignature
}

func (qc *QuorumCertificate) BytesForSig() []byte {
	return voteBytesForSig(qc.View, qc.BlockMRH)
}
