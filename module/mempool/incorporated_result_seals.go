// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type IncorporatedResultSeals interface {
	Add(irSeal *flow.IncorporatedResultSeal) bool

	All() []*flow.IncorporatedResultSeal

	ByID(flow.Identifier) (*flow.IncorporatedResultSeal, bool)

	Limit() uint

	Rem(incorporatedResultID flow.Identifier) bool

	Size() uint
}
