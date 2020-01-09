package types

type QuorumCertificate struct {
	View                uint64
	BlockMRH            MRH
	AggregatedSignature *AggregatedSignature
}
