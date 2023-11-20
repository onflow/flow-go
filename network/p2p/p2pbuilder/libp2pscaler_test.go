package p2pbuilder

import (
	"testing"

	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/pbnjay/memory"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAllowedMemoryScale(t *testing.T) {
	m := memory.TotalMemory()
	require.True(t, m > 0)

	// scaling with factor of 1 should return the total memory.
	s, err := allowedMemory(1)
	require.NoError(t, err)
	require.Equal(t, int64(m), s)

	// scaling with factor of 0 should return an error.
	_, err = allowedMemory(0)
	require.Error(t, err)

	// scaling with factor of -1 should return an error.
	_, err = allowedMemory(-1)
	require.Error(t, err)

	// scaling with factor of 2 should return an error.
	_, err = allowedMemory(2)
	require.Error(t, err)

	// scaling with factor of 0.5 should return half the total memory.
	s, err = allowedMemory(0.5)
	require.NoError(t, err)
	require.Equal(t, int64(m/2), s)

	// scaling with factor of 0.1 should return 10% of the total memory.
	s, err = allowedMemory(0.1)
	require.NoError(t, err)
	require.Equal(t, int64(m/10), s)

	// scaling with factor of 0.01 should return 1% of the total memory.
	s, err = allowedMemory(0.01)
	require.NoError(t, err)
	require.Equal(t, int64(m/100), s)

	// scaling with factor of 0.001 should return 0.1% of the total memory.
	s, err = allowedMemory(0.001)
	require.NoError(t, err)
	require.Equal(t, int64(m/1000), s)

	// scaling with factor of 0.0001 should return 0.01% of the total memory.
	s, err = allowedMemory(0.0001)
	require.NoError(t, err)
	require.Equal(t, int64(m/10000), s)
}

func TestAllowedFileDescriptorsScale(t *testing.T) {
	// getting actual file descriptor limit.
	fd, err := getNumFDs()
	require.NoError(t, err)
	require.True(t, fd > 0)

	// scaling with factor of 1 should return the total file descriptors.
	s, err := allowedFileDescriptors(1)
	require.NoError(t, err)
	require.Equal(t, fd, s)

	// scaling with factor of 0 should return an error.
	_, err = allowedFileDescriptors(0)
	require.Error(t, err)

	// scaling with factor of -1 should return an error.
	_, err = allowedFileDescriptors(-1)
	require.Error(t, err)

	// scaling with factor of 2 should return an error.
	_, err = allowedFileDescriptors(2)
	require.Error(t, err)

	// scaling with factor of 0.5 should return half the total file descriptors.
	s, err = allowedFileDescriptors(0.5)
	require.NoError(t, err)
	require.Equal(t, fd/2, s)

	// scaling with factor of 0.1 should return 10% of the total file descriptors.
	s, err = allowedFileDescriptors(0.1)
	require.NoError(t, err)
	require.Equal(t, fd/10, s)

	// scaling with factor of 0.01 should return 1% of the total file descriptors.
	s, err = allowedFileDescriptors(0.01)
	require.NoError(t, err)
	require.Equal(t, fd/100, s)

	// scaling with factor of 0.001 should return 0.1% of the total file descriptors.
	s, err = allowedFileDescriptors(0.001)
	require.NoError(t, err)
	require.Equal(t, fd/1000, s)

	// scaling with factor of 0.0001 should return 0.01% of the total file descriptors.
	s, err = allowedFileDescriptors(0.0001)
	require.NoError(t, err)
	require.Equal(t, fd/10000, s)
}

// TestApplyResourceLimitOverride tests the ApplyResourceLimitOverride function. It tests the following cases:
// 1. The override limit is not set (i.e., left at zero), the original limit should be used.
// 2. The override limit is set, the override limit should be used.
func TestApplyResourceLimitOverride(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	mem, err := allowedMemory(cfg.NetworkConfig.ResourceManager.MemoryLimitRatio)
	require.NoError(t, err)

	fd, err := allowedFileDescriptors(cfg.NetworkConfig.ResourceManager.FileDescriptorsRatio)
	require.NoError(t, err)
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	scaled := limits.Scale(mem, fd)

	systemOverride := p2pconf.ResourceManagerOverrideLimit{
		StreamsInbound:      0, // should not be overridden.
		StreamsOutbound:     456,
		ConnectionsInbound:  789,
		ConnectionsOutbound: 0, // should not be overridden.
		FD:                  4560,
		Memory:              7890,
	}

	peerOverride := p2pconf.ResourceManagerOverrideLimit{
		StreamsInbound:      321,
		StreamsOutbound:     0, // should not be overridden.
		ConnectionsInbound:  987,
		ConnectionsOutbound: 3210,
		FD:                  0, // should not be overridden.
		Memory:              9870,
	}

	partial := rcmgr.PartialLimitConfig{}
	partial.System = ApplyResourceLimitOverride(unittest.Logger(), p2pconf.ResourceScopeSystem, scaled.ToPartialLimitConfig().System, systemOverride)
	partial.PeerDefault = ApplyResourceLimitOverride(unittest.Logger(), p2pconf.ResourceScopePeer, scaled.ToPartialLimitConfig().PeerDefault, peerOverride)

	final := partial.Build(scaled).ToPartialLimitConfig()
	require.Equal(t, 456, int(final.System.StreamsOutbound))                                           // should be overridden.
	require.Equal(t, 789, int(final.System.ConnsInbound))                                              // should be overridden.
	require.Equal(t, 4560, int(final.System.FD))                                                       // should be overridden.
	require.Equal(t, 7890, int(final.System.Memory))                                                   // should be overridden.
	require.Equal(t, scaled.ToPartialLimitConfig().System.StreamsInbound, final.System.StreamsInbound) // should NOT be overridden.
	require.Equal(t, scaled.ToPartialLimitConfig().System.ConnsOutbound, final.System.ConnsOutbound)   // should NOT be overridden.
}

// TestBuildLibp2pResourceManagerLimits tests the BuildLibp2pResourceManagerLimits function.
// It creates a default configuration and an overridden configuration, and tests overriding the default configuration.
func TestBuildLibp2pResourceManagerLimits(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// the default concrete limits is built from the default configuration.
	defaultConcreteLimits, err := BuildLibp2pResourceManagerLimits(unittest.Logger(), &cfg.NetworkConfig.ResourceManager)
	require.NoError(t, err)

	// now the test creates random override configs for each scope, and re-build the concrete limits.
	cfg.NetworkConfig.ResourceManager.Override.System = unittest.LibP2PResourceLimitOverrideFixture()
	cfg.NetworkConfig.ResourceManager.Override.Transient = unittest.LibP2PResourceLimitOverrideFixture()
	cfg.NetworkConfig.ResourceManager.Override.Protocol = unittest.LibP2PResourceLimitOverrideFixture()
	cfg.NetworkConfig.ResourceManager.Override.Peer = unittest.LibP2PResourceLimitOverrideFixture()
	cfg.NetworkConfig.ResourceManager.Override.PeerProtocol = unittest.LibP2PResourceLimitOverrideFixture()
	overriddenConcreteLimits, err := BuildLibp2pResourceManagerLimits(unittest.Logger(), &cfg.NetworkConfig.ResourceManager)
	require.NoError(t, err)

	// this function will evaluate that the limits are override correctly on each scope using the override config.
	requireEqual := func(t *testing.T, override p2pconf.ResourceManagerOverrideLimit, actual rcmgr.ResourceLimits) {
		require.Equal(t, override.StreamsInbound, int(actual.StreamsInbound))
		require.Equal(t, override.StreamsOutbound, int(actual.StreamsOutbound))
		require.Equal(t, override.ConnectionsInbound, int(actual.ConnsInbound))
		require.Equal(t, override.ConnectionsOutbound, int(actual.ConnsOutbound))
		require.Equal(t, override.FD, int(actual.FD))
		require.Equal(t, override.Memory, int(actual.Memory))
	}

	op := overriddenConcreteLimits.ToPartialLimitConfig()
	requireEqual(t, cfg.NetworkConfig.ResourceManager.Override.System, op.System)
	requireEqual(t, cfg.NetworkConfig.ResourceManager.Override.Transient, op.Transient)
	requireEqual(t, cfg.NetworkConfig.ResourceManager.Override.Protocol, op.ProtocolDefault)
	requireEqual(t, cfg.NetworkConfig.ResourceManager.Override.Peer, op.PeerDefault)
	requireEqual(t, cfg.NetworkConfig.ResourceManager.Override.PeerProtocol, op.ProtocolPeerDefault)

	// this function will evaluate that the default limits (before overriding) are not equal to the overridden limits.
	requireNotEqual := func(t *testing.T, a rcmgr.ResourceLimits, b rcmgr.ResourceLimits) {
		require.NotEqual(t, a.StreamsInbound, b.StreamsInbound)
		require.NotEqual(t, a.StreamsOutbound, b.StreamsOutbound)
		require.NotEqual(t, a.ConnsInbound, b.ConnsInbound)
		require.NotEqual(t, a.ConnsOutbound, b.ConnsOutbound)
		require.NotEqual(t, a.FD, b.FD)
		require.NotEqual(t, a.Memory, b.Memory)
	}
	dp := defaultConcreteLimits.ToPartialLimitConfig()
	requireNotEqual(t, dp.System, op.System)
	requireNotEqual(t, dp.Transient, op.Transient)
	requireNotEqual(t, dp.ProtocolDefault, op.ProtocolDefault)
	requireNotEqual(t, dp.PeerDefault, op.PeerDefault)
	requireNotEqual(t, dp.ProtocolPeerDefault, op.ProtocolPeerDefault)
}
