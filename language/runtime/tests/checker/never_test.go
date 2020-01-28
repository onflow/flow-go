package checker

import (
	"testing"

	require "github.com/stretchr/testify/require"

	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckNever(t *testing.T) {

	_, err := ParseAndCheckWithPanic(t,
		`
            pub fun test(): Int {
                return panic("XXX")
            }
        `,
	)

	require.NoError(t, err)
}
