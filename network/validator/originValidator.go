package validator

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network"
)

var _ network.MessageValidator = &OriginValidator{}

// OriginValidator returns true if the sender of the message is among the set of identifiers
// returned by the given IdentifierProvider
type OriginValidator struct {
	idProvider id.IdentifierProvider
}

func NewOriginValidator(provider id.IdentifierProvider) network.MessageValidator {
	return &OriginValidator{provider}
}

func (v OriginValidator) Validate(origin flow.Identifier, msg interface{}) bool {
	return v.idProvider.Identifiers().Contains(origin)
}
