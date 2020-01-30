package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

type Vote struct {
	UnsignedVote
	Signature *Signature
}

func (v *Vote) ID() []byte {
	data := encoding.DefaultEncoder.MustEncode(v)
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	hash := hasher.ComputeHash(data)
	return hash
}

func NewVote(unsigned *UnsignedVote, sig *Signature) *Vote {
	return &Vote{
		UnsignedVote: *unsigned,
		Signature:    sig,
	}
}

func (uv Vote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockID)
}
