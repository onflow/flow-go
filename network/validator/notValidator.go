package validator

import (
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &NotValidator{}

type NotValidator struct {
	validator network.MessageValidator
}

func NewNotValidator(validator network.MessageValidator) *NotValidator {
	return &NotValidator{
		validator: validator,
	}
}

func (n NotValidator) Validate(msg message.Message) bool {
	return !n.validator.Validate(msg)
}

