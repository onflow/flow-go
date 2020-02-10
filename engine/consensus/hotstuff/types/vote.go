package types

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Vote struct {
	UnsignedVote
	Signature *Signature
}

func (v *Vote) ID() flow.Identifier {
	// TODO: replace this to generic hash function
	data := encoding.DefaultEncoder.MustEncode(v)
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	hash := hasher.ComputeHash(data)
	return flow.HashToID(hash)
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
