package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p/config"
)

// BuildLibp2pResourceManagerLimits builds the resource manager limits for the libp2p node.
// Args:
//
//	logger: logger to log the resource manager limits.
//	config: the resource manager configuration.
//
// Returns:
//
//   - the resource manager limits.
//   - any error encountered, all returned errors are irrecoverable (we cannot continue without the resource manager limits).
func BuildLibp2pResourceManagerLimits(logger zerolog.Logger, config *p2pconfig.ResourceManagerConfig) (*rcmgr.ConcreteLimitConfig, error) {
	defaultLimits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&defaultLimits)

	mem, err := allowedMemory(config.MemoryLimitRatio)
	if err != nil {
		return nil, fmt.Errorf("could not get allowed memory: %w", err)
	}
	fd, err := allowedFileDescriptors(config.FileDescriptorsRatio)
	if err != nil {
		return nil, fmt.Errorf("could not get allowed file descriptors: %w", err)
	}

	scaled := defaultLimits.Scale(mem, fd)
	scaledP := scaled.ToPartialLimitConfig()
	override := rcmgr.PartialLimitConfig{}
	override.System = ApplyResourceLimitOverride(logger, p2pconfig.ResourceScopeSystem, scaledP.System, config.Override.System)
	override.Transient = ApplyResourceLimitOverride(logger, p2pconfig.ResourceScopeTransient, scaledP.Transient, config.Override.Transient)
	override.ProtocolDefault = ApplyResourceLimitOverride(logger, p2pconfig.ResourceScopeProtocol, scaledP.ProtocolDefault, config.Override.Protocol)
	override.PeerDefault = ApplyResourceLimitOverride(logger, p2pconfig.ResourceScopePeer, scaledP.PeerDefault, config.Override.Peer)
	override.ProtocolPeerDefault = ApplyResourceLimitOverride(logger, p2pconfig.ResourceScopePeerProtocol, scaledP.ProtocolPeerDefault, config.Override.PeerProtocol)

	limits := override.Build(scaled)
	logger.Info().
		Str("key", keyResourceManagerLimit).
		Int64("allowed_memory", mem).
		Int("allowed_file_descriptors", fd).
		Msg("allowed memory and file descriptors are fetched from the system")
	newLimitConfigLogger(logger.With().Str("key", keyResourceManagerLimit).Logger()).LogResourceManagerLimits(limits)
	return &limits, nil
}

// ApplyResourceLimitOverride applies the override limit to the original limit.
// For any attribute that is set in the override limit, the original limit will be overridden, otherwise
// the original limit will be used.
// Args:
//
//		logger: logger to log the override limit.
//		resourceScope: the scope of the resource, e.g., system, transient, protocol, peer, peer-protocol.
//		original: the original limit.
//	 override: the override limit.
//
// Returns:
//
//	the overridden limit.
func ApplyResourceLimitOverride(logger zerolog.Logger,
	resourceScope p2pconfig.ResourceScope,
	original rcmgr.ResourceLimits,
	override p2pconfig.ResourceManagerOverrideLimit) rcmgr.ResourceLimits {
	lg := logger.With().Logger()

	if override.StreamsInbound > 0 {
		lg = lg.With().Int("streams-inbound-override", override.StreamsInbound).Int("streams-inbound-original", int(original.StreamsInbound)).Logger()
		original.StreamsInbound = rcmgr.LimitVal(override.StreamsInbound)
	}

	if override.StreamsOutbound > 0 {
		lg = lg.With().Int("streams-outbound-override", override.StreamsOutbound).Int("streams-outbound-original", int(original.StreamsOutbound)).Logger()
		original.StreamsOutbound = rcmgr.LimitVal(override.StreamsOutbound)
	}

	if override.ConnectionsInbound > 0 {
		lg = lg.With().Int("connections-inbound-override", override.ConnectionsInbound).Int("connections-inbound-original", int(original.ConnsInbound)).Logger()
		original.ConnsInbound = rcmgr.LimitVal(override.ConnectionsInbound)
	}

	if override.ConnectionsOutbound > 0 {
		lg = lg.With().Int("connections-outbound-override", override.ConnectionsOutbound).Int("connections-outbound-original", int(original.ConnsOutbound)).Logger()
		original.ConnsOutbound = rcmgr.LimitVal(override.ConnectionsOutbound)
	}

	if override.FD > 0 {
		lg = lg.With().Int("fd-override", override.FD).Int("fd-original", int(original.FD)).Logger()
		original.FD = rcmgr.LimitVal(override.FD)
	}

	if override.Memory > 0 {
		lg = lg.With().Int("memory-override", override.Memory).Int("memory-original", int(original.Memory)).Logger()
		original.Memory = rcmgr.LimitVal64(override.Memory)
	}

	lg.Info().Str("scope", resourceScope.String()).Msg("scope resource limits examined for override")
	return original
}
