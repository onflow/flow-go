package verification

import (
	"encoding/binary"
	
	"github.com/onflow/flow-go/model/flow"
)

// MakeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func MakeVoteMessage(view uint64, blockID flow.Identifier) []byte {
	msg := make([]byte, 8, 8+flow.IdentifierLen)
	binary.BigEndian.PutUint64(msg, view)
	msg = append(msg, blockID[:]...)
	return msg
}

// MakeTimeoutMessage generates the message we have to sign in order to be able
// to contribute to Active Pacemaker protocol. Each replica signs with the highest QC view
// known to that replica.
func MakeTimeoutMessage(view uint64, highQCView uint64) []byte {
	msg := make([]byte, 16)
	binary.BigEndian.PutUint64(msg[:8], view)
	binary.BigEndian.PutUint64(msg[8:], highQCView)
	return msg
}
