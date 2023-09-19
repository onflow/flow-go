package unittest

import (
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"
)

func RunWithConfiguredPebbleInstance(tb testing.TB, opts *pebble.Options, f func(s *pebble.DB)) {
	s, dbPath := TempPebbleDBWithOpts(tb, opts)
	defer func() {
		require.NoError(tb, s.Close())
		require.NoError(tb, os.RemoveAll(dbPath))
	}()
	f(s)
}
