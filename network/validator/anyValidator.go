package validator

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
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

func (v AnyValidator) Validate(origin flow.Identifier, msg interface{}) bool {
	for _, validator := range v.validators {
		if validator.Validate(origin, msg) {
			return true
		}
	}
	return false
}
