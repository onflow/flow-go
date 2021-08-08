package validator

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &NotSenderValidator{}

// SenderValidator validates that the message has not been sent by the given sender ID
type NotSenderValidator struct {
	sender []byte
}

// NewNotSenderValidator creates and returns a new NotSenderValidator for the given sender ID
func NewNotSenderValidator(sender flow.Identifier) *SenderValidator {
	sv := NewSenderValidator(sender)
	return NewNotSenderValidator(sv)
}

// Validate returns true if the message origin id is different from the sender ID.
func (sv *SenderValidator) Validate(msg message.Message) bool {
	return !bytes.Equal(sv.sender, msg.OriginID)
}
