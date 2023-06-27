package validator

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

var _ network.MessageValidator = &SenderValidator{}

// SenderValidator validates messages by sender ID
type SenderValidator struct {
	sender flow.Identifier
}

var _ network.MessageValidator = (*SenderValidator)(nil)

// ValidateSender creates and returns a new SenderValidator for the given sender ID
func ValidateSender(sender flow.Identifier) network.MessageValidator {
	sv := &SenderValidator{}
	sv.sender = sender
	return sv
}

// Validate returns true if the message origin id is the same as the sender ID.
func (sv *SenderValidator) Validate(msg network.IncomingMessageScope) bool {
	return sv.sender == msg.OriginId()
}

// ValidateNotSender creates and returns a validator which validates that the message origin id is different from
// sender id
func ValidateNotSender(sender flow.Identifier) network.MessageValidator {
	return NewNotValidator(ValidateSender(sender))
}
