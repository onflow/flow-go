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

// TestApplyInboundStreamLimits tests that the inbound stream limits are applied correctly, i.e., the limits from the config file
// are applied to the concrete limit config when the concrete limit config is greater than the limits from the config file.
func TestApplyInboundStreamAndConnectionLimits(t *testing.T) {
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
