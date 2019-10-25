// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// Role filters nodes for the given roles.
func Role(roles ...string) function.NodeFilter {
	lookup := make(map[string]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(n *model.Node) bool {
		_, ok := lookup[n.Role]
		return ok
	}
}
