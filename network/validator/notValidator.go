package validator

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
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

func (n NotValidator) Validate(origin flow.Identifier, msg interface{}) bool {
	return !n.validator.Validate(origin, msg)
}
