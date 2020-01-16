package types

import "encoding/binary"

type UnsignedVote struct {
	View     uint64
	BlockMRH []byte
}

func NewUnsignedVote(view uint64, blockMRH []byte) *Vote {
	return &Vote{
		View:     view,
		BlockMRH: blockMRH,
	}
}

func (uv *UnsignedVote) BytesForSig() []byte {
	// TODO there's probably a cleaner way to do this whole fcn
	voteStrBytes := []byte("vote")

	viewBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewBytes, uv.View)

	length := len(viewBytes) + len(uv.BlockMRH) + len(voteStrBytes)
	bytesForSig := make([]byte, 0, length)
	bytesForSig = append(bytesForSig, voteStrBytes...)
	bytesForSig = append(bytesForSig, viewBytes...)
	bytesForSig = append(bytesForSig, uv.BlockMRH...)

	return bytesForSig
}
