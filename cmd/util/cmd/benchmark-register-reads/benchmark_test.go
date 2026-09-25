package benchmark_register_reads

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func TestSplitKeys(t *testing.T) {
	keys := make([]registerKey, 10)
	for i := range keys {
		keys[i] = registerKey{id: flow.RegisterID{Owner: "owner", Key: string(rune('a' + i))}}
	}

	shares := splitKeys(keys, 4)
	require.Len(t, shares, 4)

	seen := 0
	for _, share := range shares {
		require.LessOrEqual(t, len(share), 3)
		seen += len(share)
	}
	require.Equal(t, len(keys), seen)
	require.Equal(t, keys, append(append(append(shares[0], shares[1]...), shares[2]...), shares[3]...))
}

func TestSampleEvery(t *testing.T) {
	keys := make([]registerKey, 100)
	for i := range keys {
		keys[i] = registerKey{id: flow.RegisterID{Key: string(rune(i))}}
	}

	require.Equal(t, keys, sampleEvery(keys, 100))
	require.Equal(t, keys, sampleEvery(keys, 200))

	sampled := sampleEvery(keys, 10)
	require.Len(t, sampled, 10)
	require.Equal(t, keys[0], sampled[0])
	require.Equal(t, keys[90], sampled[9])
}

func TestPercentile(t *testing.T) {
	require.Zero(t, percentile(nil, 0.5))

	durations := []time.Duration{1, 2, 3, 4, 5}
	require.Equal(t, time.Duration(1), percentile(durations, 0))
	require.Equal(t, time.Duration(3), percentile(durations, 0.5))
	require.Equal(t, time.Duration(5), percentile(durations, 1))
}

func TestKeyStream(t *testing.T) {
	keys := []registerKey{
		{id: flow.RegisterID{Owner: "owner", Key: "a"}},
		{id: flow.RegisterID{Owner: "owner", Key: "b"}},
	}

	// without missing registers, every read is one of the given registers
	stream, err := keyStream(keys, 100, 0, 1, 0)
	require.NoError(t, err)
	require.Len(t, stream, 100)
	for _, key := range stream {
		require.Contains(t, keys, key)
	}

	// with missing registers, the missing keys make up miss-percent of the reads and are not part
	// of the state
	stream, err = keyStream(keys, 100, 10, 1, 0)
	require.NoError(t, err)
	require.Len(t, stream, 100)

	missing := 0
	for _, key := range stream {
		if key.id.Key != "a" && key.id.Key != "b" {
			missing++
			require.Contains(t, key.id.Key, "benchmark missing key")
			require.NotEmpty(t, key.id.Owner)
			require.NotEqual(t, ledger.DummyPath, key.path)
		}
	}
	require.Equal(t, 10, missing)
}

func TestCompareReaders(t *testing.T) {
	keys := []registerKey{
		{id: flow.RegisterID{Owner: "owner", Key: "a"}},
		{id: flow.RegisterID{Owner: "owner", Key: "b"}},
	}

	reader := func(name string, read func(registerKey) ([]byte, bool, error)) registerReader {
		return registerReader{name: name, read: read}
	}

	sameValues := func(key registerKey) ([]byte, bool, error) {
		return []byte("value " + key.id.Key), true, nil
	}

	mismatches, _, err := compareReaders(keys, []registerReader{
		reader("first", sameValues),
		reader("second", sameValues),
	})
	require.NoError(t, err)
	require.Zero(t, mismatches)

	// a different value is a mismatch
	mismatches, firstMismatch, err := compareReaders(keys, []registerReader{
		reader("first", sameValues),
		reader("second", func(registerKey) ([]byte, bool, error) { return []byte("other"), true, nil }),
	})
	require.NoError(t, err)
	require.Equal(t, 2, mismatches)
	require.Contains(t, firstMismatch, "first")

	// a register that only one of the readers finds is a mismatch
	mismatches, _, err = compareReaders(keys, []registerReader{
		reader("first", sameValues),
		reader("second", func(registerKey) ([]byte, bool, error) { return nil, false, nil }),
	})
	require.NoError(t, err)
	require.Equal(t, 2, mismatches)
}
