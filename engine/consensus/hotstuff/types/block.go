package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type Block struct {
	View        uint64
	QC          *QuorumCertificate
	Height      uint64
	ChainID     string
	PayloadHash []byte
	Timestamp   *timestamp.Timestamp
}

func (b *Block) BlockMRH() []byte {
	alg, _ := crypto.NewHasher(crypto.SHA3_256)
	// TODO: will integrate other fields in the future
	msgHash := alg.ComputeHash(b.PayloadHash)
	return msgHash
}
