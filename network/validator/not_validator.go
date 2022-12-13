package validator

import (
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &NotValidator{}

// NotValidator returns the opposite result of the given validator for the Validate call
type NotValidator struct {
	validator network.MessageValidator
}

func NewNotValidator(validator network.MessageValidator) network.MessageValidator {
	return &NotValidator{
		validator: validator,
	}
}

func (n NotValidator) Validate(msg message.Message) bool {
	return !n.validator.Validate(msg)
}
