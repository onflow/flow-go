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

// TestValidIdentifierListReadAsCmdLineArg tests that a valid Identifier list can be read as a command line argument
// using IdentifierListValue
func TestValidIdentifierListReadAsCmdLineArg(t *testing.T) {
	var flags flag.FlagSet
	flags.Init("test", flag.ContinueOnError)

	var ids flow.IdentifierList = unittest.IdentifierListFixture(5)
	idsAsArg := strings.Join(ids.Strings(), ",")

	var nodeIDargs IdentifierListValue
	flags.Var(&nodeIDargs, "nodeids", "usage")

	err := flags.Parse([]string{"-nodeids", idsAsArg})
	assert.NilError(t, err)

	for i, id := range nodeIDargs {
		assert.Equal(t, ids[i], id)
	}

	expect := fmt.Sprintf("%v", ids.Strings())
	assert.Equal(t, expect, nodeIDargs.String())
}

// TestInvalidIdentifierListReadAsCmdLineArg tests that an invalid Identifier list results in an error when read
// using IdentifierListValue
func TestInvalidIdentifierListReadAsCmdLineArg(t *testing.T) {
	var flags flag.FlagSet
	flags.Init("test", flag.ContinueOnError)

	invalidID := "1234"

	var nodeIDargs IdentifierListValue
	flags.Var(&nodeIDargs, "nodeids", "usage")

	if err := flags.Parse([]string{"-nodeids", invalidID}); err == nil {
		t.Fail()
	}
}
