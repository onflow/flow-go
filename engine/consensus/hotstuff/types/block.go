package types

type Block struct {
	View        uint64
	QC          *QuorumCertificate
	PayloadHash []byte
}

func (b *Block) BlockMRH() []byte {
	panic("TODO")
}

func NewBlock(view uint64, qc *QuorumCertificate, payloadHash []byte) *Block {
	return &Block{
		View:        view,
		QC:          qc,
		PayloadHash: payloadHash,
	}
}
