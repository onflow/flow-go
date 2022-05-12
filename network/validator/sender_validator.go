package validator

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &SenderValidator{}

// SenderValidator validates messages by sender ID
type SenderValidator struct {
	sender []byte
}

// ValidateSender creates and returns a new SenderValidator for the given sender ID
func ValidateSender(sender flow.Identifier) network.MessageValidator {
	sv := &SenderValidator{}
	sv.sender = sender[:]
	return sv
}

// Validate returns true if the message origin id is the same as the sender ID.
func (sv *SenderValidator) Validate(msg message.Message) bool {
	return bytes.Equal(sv.sender, msg.OriginID)
}

// ValidateNotSender creates and returns a validator which validates that the message origin id is different from
// sender id
func ValidateNotSender(sender flow.Identifier) network.MessageValidator {
	return NewNotValidator(ValidateSender(sender))
}
