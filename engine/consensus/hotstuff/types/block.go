package types

import "github.com/golang/protobuf/ptypes/timestamp"

type Block struct {
	View        uint64
	QC          *QuorumCertificate
	Height      uint64
	ChainID     string
	PayloadHash []byte
	BlockMRH    []byte
	Timestamp   *timestamp.Timestamp
}

func (b *Block) computeBlockMRH() []byte {
	panic("TODO")
}
