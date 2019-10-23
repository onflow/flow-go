// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// Committee represents an interface to the node identity table.
type Committee interface {
	Me() *model.Node
	Get(nodeID string) (*model.Node, error)
	Select(filters ...function.NodeFilter) (model.NodeList, error)
}
