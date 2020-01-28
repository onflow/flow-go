package types

import (
	"time"

	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/hash"
)

// Block represents a block in the HotStuff consensus algorithm.
type Block struct {
	// View is the view number for this block.
	View uint64

	// Justify is the quorum certificate used to justify this block as a valid
	// extension of the chain.
	Justify *QuorumCertificate

	// PayloadHash is a hash of the payload of this block. The payload contents
	// are not specified or checked by the HotStuff core algorithm.
	PayloadHash []byte
}

func (b *Block) MerkleHash() []byte {
	enc := encoding.DefaultEncoder.MustEncode(b)
	return hash.DefaultHasher.ComputeHash(enc)
}

func NewBlock(view uint64, qc *QuorumCertificate, payloadHash []byte, height uint64, chainID string) *Block {

	t := time.Now()

	return &Block{
		View:        view,
		QC:          qc,
		PayloadHash: payloadHash,
		Height:      height,
		ChainID:     chainID,
		Timestamp:   t,
	}
}
