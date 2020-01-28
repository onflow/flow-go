package checker

import (
	"testing"

	"github.com/stretchr/testify/require"
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
