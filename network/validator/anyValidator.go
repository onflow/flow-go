package validator

import (
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &AnyValidator{}

// AnyValidator returns true if any of the given validators returns true
type AnyValidator struct {
	validators []network.MessageValidator
}

func NewAnyValidator(validators ...network.MessageValidator) network.MessageValidator {
	return &AnyValidator{
		validators: validators,
	}
}

func (v AnyValidator) Validate(msg message.Message) bool {
	for _, validator := range v.validators {
		if validator.Validate(msg) {
			return true
		}
	}
	return false
}
