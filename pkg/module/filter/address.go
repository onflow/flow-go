// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// Address filters nodes for the given addresses.
func Address(addresses ...string) function.NodeFilter {
	lookup := make(map[string]struct{})
	for _, address := range addresses {
		lookup[address] = struct{}{}
	}
	return func(n *model.Node) bool {
		_, ok := lookup[n.Address]
		return ok
	}
}
