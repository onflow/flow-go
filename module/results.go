package module

import "github.com/dapperlabs/flow-go/model/flow"

type PendingResults interface {
	Add(result *flow.PendingResult) bool
	Has(resultID flow.Identifier) bool
	Rem(resultID flow.Identifier) bool
	ByID(resultID flow.Identifier) (*flow.PendingResult, bool)
	Size() uint
}
