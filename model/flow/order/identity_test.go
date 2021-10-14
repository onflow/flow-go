// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"encoding/binary"

	"github.com/onflow/flow-go/model/flow"
)

func Test_NodeIDAsc(t *testing.T) {

	// create some identities to sort
	ident := new(flow.Identity)
	ident.NodeID = flow.Identifier{1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4}

	ident2 := new(flow.Identity)
	ident2.NodeID = flow.Identifier{1, 1, 1, 1, 1, 1, 1, 1, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5}

	assert.Equal(t, ByNodeIDAsc(ident, ident2), true)

	ident3 := new(flow.Identity)
	ident3.NodeID = flow.Identifier{1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4}

	ident4 := new(flow.Identity)
	ident4.NodeID = flow.Identifier{1, 1, 1, 9, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4}

	ident5 := new(flow.Identity)
	ident5.NodeID = flow.Identifier{1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 5}

	// is ident less than ident3
	assert.Equal(t, ByNodeIDAsc(ident, ident3), false)

	// is ident3 less than ident4
	assert.Equal(t, ByNodeIDAsc(ident3, ident4), true)

	// is ident4 less than ident3
	assert.Equal(t, ByNodeIDAsc(ident4, ident3), false)

	// is ident3 less than ident
	assert.Equal(t, ByNodeIDAsc(ident3, ident), false)

	// is ident2 less than ident
	assert.Equal(t, ByNodeIDAsc(ident2, ident), false)

	// is ident3 less than ident5
	assert.Equal(t, ByNodeIDAsc(ident3, ident5), true)
}

func ByNodeIDAsc(identity1 *flow.Identity, identity2 *flow.Identity) bool {
	num1 := identity1.NodeID[:]
	num2 := identity2.NodeID[:]
	first := binary.BigEndian.Uint64(num1)
	second := binary.BigEndian.Uint64(num2)

	if first == second {
		num1 = num1[8:]
		num2 = num2[8:]
		first = binary.BigEndian.Uint64(num1)
		second = binary.BigEndian.Uint64(num2)

		if first == second {
			num1 = num1[8:]
			num2 = num2[8:]
			first = binary.BigEndian.Uint64(num1)
			second = binary.BigEndian.Uint64(num2)

			if first == second {
				num1 = num1[8:]
				num2 = num2[8:]
				first = binary.BigEndian.Uint64(num1)
				second = binary.BigEndian.Uint64(num2)
				return first < second
			}
			return first < second
		}
		return first < second
	}
	return first < second
}
