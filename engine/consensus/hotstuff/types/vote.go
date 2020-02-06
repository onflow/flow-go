package types

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Vote struct {
	View      uint64
	BlockID   flow.Identifier
	Signature *flow.PartialSignature
}

func (uv *Vote) ID() flow.Identifier {
	panic("TODO")
}

func (uv *Vote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockID)
}

// TODO: will be refactored in validator PR
func voteBytesForSig(view uint64, blockID flow.Identifier) []byte {
	// TODO there's probably a cleaner way to do this whole fcn
	voteStrBytes := []byte("vote")

	viewBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewBytes, view)

	length := len(voteStrBytes) + len(viewBytes) + len(blockID)
	bytesForSig := make([]byte, 0, length)
	bytesForSig = append(bytesForSig, voteStrBytes...)
	bytesForSig = append(bytesForSig, viewBytes...)
	bytesForSig = append(bytesForSig, blockID[:]...)

	return bytesForSig
}
