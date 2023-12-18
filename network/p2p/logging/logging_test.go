package p2plogging_test

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPeerIdLogging checks the end-to-end functionality of the PeerId logger helper.
// It ensures that the PeerId logger helper returns the same string as the peer.ID.String() method.
func TestPeerIdLogging(t *testing.T) {
	pid := unittest.PeerIdFixture(t)
	pidStr := p2plogging.PeerId(pid)
	require.Equal(t, pid.String(), pidStr)
}

// BenchmarkPeerIdString benchmarks the peer.ID.String() method.
func BenchmarkPeerIdString(b *testing.B) {
	unittest.SkipBenchmarkUnless(b, unittest.BENCHMARK_EXPERIMENT, "skips peer id string benchmarking, set environment variable to enable")

	count := 100
	pids := make([]peer.ID, 0, count)
	for i := 0; i < count; i++ {
		pids = append(pids, unittest.PeerIdFixture(b))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pids[i%count].String()
	}
}

// BenchmarkPeerIdLogging benchmarks the PeerId logger helper, which is expected to be faster than the peer.ID.String() method,
// as it caches the base58 encoded peer ID strings.
func BenchmarkPeerIdLogging(b *testing.B) {
	unittest.SkipBenchmarkUnless(b, unittest.BENCHMARK_EXPERIMENT, "skips peer id logging benchmarking, set environment variable to enable")

	count := 100
	pids := make([]peer.ID, 0, count)
	for i := 0; i < count; i++ {
		pids = append(pids, unittest.PeerIdFixture(b))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p2plogging.PeerId(pids[i%count])
	}
}
