package validator

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

var _ network.MessageValidator = (*OriginValidator)(nil)

// OriginValidator returns true if the sender of the message is among the set of identifiers
// returned by the given IdentifierProvider
type OriginValidator struct {
	idProvider module.IdentifierProvider
}

func NewOriginValidator(provider module.IdentifierProvider) network.MessageValidator {
	return &OriginValidator{provider}
}

func (v OriginValidator) Validate(msg network.IncomingMessageScope) bool {
	return v.idProvider.Identifiers().Contains(msg.OriginId())
}
