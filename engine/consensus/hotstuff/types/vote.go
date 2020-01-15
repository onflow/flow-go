package types

import "encoding/binary"

type Vote struct {
	View      uint64
	BlockMRH  MRH
	Signature *Signature
}

func NewVote(view uint64, blockMRH MRH, sig *Signature) *Vote {
	return &Vote{
		View:      view,
		BlockMRH:  blockMRH,
		Signature: sig,
	}
}

func (uv *Vote) BytesForSig() []byte {
	// TODO there's probably a cleaner way to do this whole fcn
	viewBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewBytes, uv.View)

	voteStrBytes := []byte("vote")

	length := len(viewBytes) + len(uv.BlockMRH) + len(voteStrBytes)
	bytesForSig := make([]byte, 0, length)
	bytesForSig = append(bytesForSig, viewBytes...)
	bytesForSig = append(bytesForSig, uv.BlockMRH[:]...)
	bytesForSig = append(bytesForSig, voteStrBytes...)

	return bytesForSig
}
