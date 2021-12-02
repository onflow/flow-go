package verification

import (
	"github.com/onflow/flow-go/model/flow"
)

// MakeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func MakeVoteMessage(view uint64, blockID flow.Identifier) []byte {
	msg := flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: blockID,
		View:    view,
	})
	return msg[:]
}
