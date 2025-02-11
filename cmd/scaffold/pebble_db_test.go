package scaffold_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/cmd/scaffold"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPebbleDB(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		_, closer, err := scaffold.InitPebbleDB(dir)
		require.NoError(t, err)
		require.NoError(t, closer.Close())
	})
}

func TestInitPebbleDBDirNotSet(t *testing.T) {
	_, _, err := scaffold.InitPebbleDB(cmd.NotSet)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required flag")
}
