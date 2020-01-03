package types

type Block struct {
	View       uint64
	QC         *QuorumCertificate
	PayloadMRH MRH
}

func (b *Block) BlockMRH() MRH {
	panic("TODO")
}
