package dkg

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// PublicData is the public data for DKG participants who generated their key shares
type PublicData struct {
	GroupPubKey     crypto.PublicKey                 // the group public key
	IDToParticipant map[flow.Identifier]*Participant // the mapping from node identifier to participant info
}
