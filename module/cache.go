package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type PayloadCache interface {
	AddGuarantees(height uint64, guaranteeIDs []flow.Identifier)
	AddSeals(height uint64, sealIDs []flow.Identifier)
	HasGuarantee(guaranteeID flow.Identifier) bool
	HasSeal(sealID flow.Identifier) bool
}
