package validator

import (
	"github.com/onflow/flow-go/network"
)

var _ network.MessageValidator = (*AnyValidator)(nil)

// AnyValidator returns true if any of the given validators returns true
type AnyValidator struct {
	validators []network.MessageValidator
}

func NewAnyValidator(validators ...network.MessageValidator) network.MessageValidator {
	return &AnyValidator{
		validators: validators,
	}
}

func (v AnyValidator) Validate(msg network.IncomingMessageScope) bool {
	for _, validator := range v.validators {
		if validator.Validate(msg) {
			return true
		}
	}
	return false
}
