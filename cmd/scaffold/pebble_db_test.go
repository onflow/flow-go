package scaffold

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPebbleDB(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		_, closer, err := InitPebbleDB(dir)
		require.NoError(t, err)
		require.NoError(t, closer.Close())
	})
}
