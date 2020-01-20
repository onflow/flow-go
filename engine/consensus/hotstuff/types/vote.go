package types

import (
	"bytes"
	"crypto/sha256"
)

type Vote struct {
	View      uint64
	BlockMRH  []byte
	Signature *Signature
}

func (v *Vote) Hash() string {
	data := bytes.Join(
		[][]byte{
			make([]byte, v.View),
			v.BlockMRH,
			v.Signature.RawSignature[:],
			make([]byte, v.Signature.SignerIdx),
		},
		[]byte{},
	)

	voteHash := sha256.Sum256(data)

	return string(voteHash[:])
}
