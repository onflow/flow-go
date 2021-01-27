package unittest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func RequireFileEmpty(t *testing.T, path string) {
	require.FileExists(t, path)

	fi, err := os.Stat(path)
	require.NoError(t, err)

	size := fi.Size()

	require.Equal(t, int64(0), size)
}
