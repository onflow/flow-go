package p2pbuilder

import (
	"testing"

	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/pbnjay/memory"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
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
func TestApplyInboundStreamLimits(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	mem, err := allowedMemory(cfg.NetworkConfig.ResourceManagerConfig.MemoryLimitRatio)
	require.NoError(t, err)

	fd, err := allowedFileDescriptors(cfg.NetworkConfig.FileDescriptorsRatio)
	require.NoError(t, err)
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	scaled := limits.Scale(mem, fd)

	concrete := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			// intentionally sets to 1 to test that it is not overridden.
			StreamsInbound: 1,
		},
		Transient: rcmgr.ResourceLimits{
			// sets it higher than the default to test that it is overridden.
			StreamsInbound: rcmgr.LimitVal(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Transient + 1),
		},
		ProtocolDefault: rcmgr.ResourceLimits{
			// sets it higher than the default to test that it is overridden.
			StreamsInbound: rcmgr.LimitVal(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Protocol + 1),
			// intentionally sets it lower than the default to test that it is not overridden.
			ConnsInbound: rcmgr.LimitVal(cfg.NetworkConfig.ResourceManagerConfig.PeerBaseLimitConnsInbound - 1),
		},
		ProtocolPeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: 1, // intentionally sets to 1 to test that it is not overridden.
		},
		PeerDefault: rcmgr.ResourceLimits{
			StreamsInbound: rcmgr.LimitVal(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Peer + 1),
		},
		Conn: rcmgr.ResourceLimits{
			StreamsInbound: 1, // intentionally sets to 1 to test that it is not overridden.
		},
		Stream: rcmgr.ResourceLimits{
			StreamsInbound: 1, // intentionally sets to 1 to test that it is not overridden.
		},
	}.Build(scaled)

	// apply inbound stream limits from the config file.
	applied := ApplyInboundStreamLimits(unittest.Logger(), concrete, cfg.NetworkConfig.ResourceManagerConfig.InboundStream)

	// then applies the peer base limit connections from the config file.
	applied = ApplyInboundConnectionLimits(unittest.Logger(), applied, cfg.NetworkConfig.ResourceManagerConfig.PeerBaseLimitConnsInbound)

	// check that the applied limits are overridden.
	// transient limit should be overridden.
	require.Equal(t, int(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Transient), int(applied.ToPartialLimitConfig().Transient.StreamsInbound))
	// protocol default limit should be overridden.
	require.Equal(t, int(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Protocol), int(applied.ToPartialLimitConfig().ProtocolDefault.StreamsInbound))
	// peer default limit should be overridden.
	require.Equal(t, int(cfg.NetworkConfig.ResourceManagerConfig.InboundStream.Peer), int(applied.ToPartialLimitConfig().PeerDefault.StreamsInbound))
	// protocol peer default limit should not be overridden.
	require.Equal(t, int(1), int(applied.ToPartialLimitConfig().ProtocolPeerDefault.StreamsInbound))
	// conn limit should not be overridden.
	require.Equal(t, int(1), int(applied.ToPartialLimitConfig().Conn.StreamsInbound))
	// stream limit should not be overridden.
	require.Equal(t, int(1), int(applied.ToPartialLimitConfig().Stream.StreamsInbound))
	// system limit should not be overridden.
	require.Equal(t, int(1), int(applied.ToPartialLimitConfig().System.StreamsInbound))
}

// TestApplyInboundConnectionLimits tests that the inbound connection limits are applied correctly, i.e., the limits from the config file
// are applied to the concrete limit config when the concrete limit config is set.
func TestApplyInboundConnectionLimits(t *testing.T) {
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	// by default the peer base limit connections is set to 0 meaning that the libp2p default is used.
	require.Equal(t, 0, int(cfg.NetworkConfig.ResourceManagerConfig.PeerBaseLimitConnsInbound))

	mem, err := allowedMemory(cfg.NetworkConfig.ResourceManagerConfig.MemoryLimitRatio)
	require.NoError(t, err)

	fd, err := allowedFileDescriptors(cfg.NetworkConfig.FileDescriptorsRatio)
	require.NoError(t, err)
	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	scaled := limits.Scale(mem, fd)

	// then applies the peer base limit connections from the config file.
	applied1 := ApplyInboundConnectionLimits(unittest.Logger(), scaled, cfg.NetworkConfig.ResourceManagerConfig.PeerBaseLimitConnsInbound)
	// since the peer base limit connections is set to 0, the libp2p default should be used, hence applied1 should be equal to scaled (i.e., the libp2p default).
	require.Equal(t, int(scaled.ToPartialLimitConfig().PeerDefault.ConnsInbound), int(applied1.ToPartialLimitConfig().PeerDefault.ConnsInbound))
	// default libp2p peer base limit connections should be greater than 0.
	require.Greater(t, int(applied1.ToPartialLimitConfig().PeerDefault.ConnsInbound), 0)

	// now set the peer base limit connections to 100, and test that it is applied correctly.
	applied2 := ApplyInboundConnectionLimits(unittest.Logger(), scaled, 100)
	require.Equal(t, int(100), int(applied2.ToPartialLimitConfig().PeerDefault.ConnsInbound))
}
