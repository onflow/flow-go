package unittest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func toHex(ids []flow.Identifier) []string {
	hex := make([]string, 0, len(ids))
	for _, id := range ids {
		hex = append(hex, id.String())
	}
	return hex
}

func IDsEqual(t *testing.T, id1, id2 []flow.Identifier) {
	require.Equal(t, toHex(id1), toHex(id2))
}
