package validators

import (
	"bytes"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
)

var _ MessageValidator = &SenderValidator{}

// SenderValidator validates messages by sender ID
type SenderValidator struct {
	sender []byte
}

// NewSenderValidator creates and returns a new SenderValidator for the given sender ID
func NewSenderValidator(sender flow.Identifier) *SenderValidator {
	sv := &SenderValidator{}
	sv.sender = sender[:]
	return sv
}

// Validate returns true if the message origin id is different from the sender ID.
func (sv *SenderValidator) Validate(msg message.Message) bool {
	return !bytes.Equal(sv.sender, msg.OriginID)
}
