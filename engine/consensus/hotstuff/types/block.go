package types

type Block struct {
	View       uint64
	QC         *QuorumCertificate
	PayloadMRH MRH
	BlockMRH   MRH
}

func (b *Block) computeBlockMRH() MRH {
	panic("TODO")
}
