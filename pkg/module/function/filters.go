// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package function

import (
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// NodeFilter is a function type that allows filtering nodes from the node
// identity table.
type NodeFilter func(*model.Node) bool
