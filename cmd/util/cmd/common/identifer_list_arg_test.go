package common

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"gotest.tools/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIdentifierListValue tests that Identifier list can be read as a command line argument
func TestIdentifierListValue(t *testing.T) {
	var flags flag.FlagSet
	flags.Init("test", flag.PanicOnError)

	var ids flow.IdentifierList = unittest.IdentifierListFixture(5)
	idsAsArg := strings.Join(ids.Strings(), ",")

	var nodeIDargs IdentifierListValue
	flags.Var(&nodeIDargs, "nodeids", "usage")
	if err := flags.Parse([]string{"-nodeids", idsAsArg}); err != nil {
		t.Error(err)
	}

	for i, id := range nodeIDargs {
		assert.Equal(t, ids[i], id)
	}

	expect := fmt.Sprintf("%v", ids.Strings())
	assert.Equal(t, expect, nodeIDargs.String())
}
