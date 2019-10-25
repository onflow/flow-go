// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/pkg/module/function"
	"github.com/dapperlabs/flow-go/pkg/module/model"
)

// Not filters nodes that are the opposite of the wrapped filter.
func Not(filter function.NodeFilter) function.NodeFilter {
	return func(n *model.Node) bool {
		return !filter(n)
	}
}
