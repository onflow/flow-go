package types

import (
	"bytes"
	"crypto/sha256"
)

type Vote struct {
	UnsignedVote
	Signature *Signature
}

func (v *Vote) Hash() string {
	data := bytes.Join(
		[][]byte{
			v.BytesForSig(),
			v.Signature.RawSignature[:],
			make([]byte, v.Signature.SignerIdx),
		},
		[]byte{},
	)

	voteHash := sha256.Sum256(data)

	return string(voteHash[:])
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
