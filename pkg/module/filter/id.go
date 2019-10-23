// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// ID ids nodes for the given roles.
func ID(ids ...string) function.NodeFilter {
	lookup := make(map[string]struct{})
	for _, id := range ids {
		lookup[id] = struct{}{}
	}
	return func(n *model.Node) bool {
		_, ok := lookup[n.ID]
		return ok
	}
}
