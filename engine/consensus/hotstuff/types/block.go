package types

type Block struct {
	View        uint64
	QC          *QuorumCertificate
	PayloadHash []byte
}

func (b *Block) BlockMRH() []byte {
	panic("TODO")
}
