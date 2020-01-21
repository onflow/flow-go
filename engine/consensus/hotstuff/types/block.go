package types

import "github.com/golang/protobuf/ptypes/timestamp"

type Block struct {
	View        uint64
	QC          *QuorumCertificate
	Height      uint64
	ChainID     string
	PayloadHash []byte
	Timestamp   *timestamp.Timestamp
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
