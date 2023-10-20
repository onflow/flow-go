package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
)

const keyResourceManagerLimit = "libp2p_resource_manager_limit"

// notEjectedPeerFilter returns a PeerFilter that will return an error if the peer is unknown or ejected.
func notEjectedPeerFilter(idProvider module.IdentityProvider) p2p.PeerFilter {
	return func(p peer.ID) error {
		if id, found := idProvider.ByPeerID(p); !found {
			return fmt.Errorf("failed to get identity of unknown peer with peer id %s", p2plogging.PeerId(p))
		} else if id.Ejected {
			return fmt.Errorf("peer %s with node id %s is ejected", p2plogging.PeerId(p), id.NodeID.String())
		}

		return nil
	}
}

type limitConfigLogger struct {
	logger zerolog.Logger
}

// NewLimitConfigLogger creates a new limitConfigLogger.
func NewLimitConfigLogger(logger zerolog.Logger) *limitConfigLogger {
	return &limitConfigLogger{logger: logger}
}

// withBaseLimit appends the base limit to the logger with the given prefix.
func (l *limitConfigLogger) withBaseLimit(prefix string, baseLimit rcmgr.ResourceLimits) zerolog.Logger {
	return l.logger.With().
		Str("key", keyResourceManagerLimit).
		Str(fmt.Sprintf("%s_streams", prefix), fmt.Sprintf("%v", baseLimit.Streams)).
		Str(fmt.Sprintf("%s_streams_inbound", prefix), fmt.Sprintf("%v", baseLimit.StreamsInbound)).
		Str(fmt.Sprintf("%s_streams_outbound", prefix), fmt.Sprintf("%v,", baseLimit.StreamsOutbound)).
		Str(fmt.Sprintf("%s_conns", prefix), fmt.Sprintf("%v", baseLimit.Conns)).
		Str(fmt.Sprintf("%s_conns_inbound", prefix), fmt.Sprintf("%v", baseLimit.ConnsInbound)).
		Str(fmt.Sprintf("%s_conns_outbound", prefix), fmt.Sprintf("%v", baseLimit.ConnsOutbound)).
		Str(fmt.Sprintf("%s_file_descriptors", prefix), fmt.Sprintf("%v", baseLimit.FD)).
		Str(fmt.Sprintf("%s_memory", prefix), fmt.Sprintf("%v", baseLimit.Memory)).Logger()
}

func (l *limitConfigLogger) LogResourceManagerLimits(config rcmgr.ConcreteLimitConfig) {
	// PartialLimit config is the same as ConcreteLimit config, but with the exported fields.
	pCfg := config.ToPartialLimitConfig()
	l.logGlobalResourceLimits(pCfg)
	l.logServiceLimits(pCfg.Service)
	l.logProtocolLimits(pCfg.Protocol)
	l.logPeerLimits(pCfg.Peer)
	l.logPeerProtocolLimits(pCfg.ProtocolPeer)
}

func (l *limitConfigLogger) logGlobalResourceLimits(config rcmgr.PartialLimitConfig) {
	lg := l.withBaseLimit("system", config.System)
	lg.Info().Msg("system limits set")

	lg = l.withBaseLimit("transient", config.Transient)
	lg.Info().Msg("transient limits set")

	lg = l.withBaseLimit("allowed_listed_system", config.AllowlistedSystem)
	lg.Info().Msg("allowed listed system limits set")

	lg = l.withBaseLimit("allowed_lister_transient", config.AllowlistedTransient)
	lg.Info().Msg("allowed listed transient limits set")

	lg = l.withBaseLimit("service_default", config.ServiceDefault)
	lg.Info().Msg("service default limits set")

	lg = l.withBaseLimit("service_peer_default", config.ServicePeerDefault)
	lg.Info().Msg("service peer default limits set")

	lg = l.withBaseLimit("protocol_default", config.ProtocolDefault)
	lg.Info().Msg("protocol default limits set")

	lg = l.withBaseLimit("protocol_peer_default", config.ProtocolPeerDefault)
	lg.Info().Msg("protocol peer default limits set")

	lg = l.withBaseLimit("peer_default", config.PeerDefault)
	lg.Info().Msg("peer default limits set")

	lg = l.withBaseLimit("connections", config.Conn)
	lg.Info().Msg("connection limits set")

	lg = l.withBaseLimit("streams", config.Stream)
	lg.Info().Msg("stream limits set")
}

func (l *limitConfigLogger) logServiceLimits(s map[string]rcmgr.ResourceLimits) {
	for sName, sLimits := range s {
		lg := l.withBaseLimit(fmt.Sprintf("service_%s", sName), sLimits)
		lg.Info().Msg("service limits set")
	}
}

func (l *limitConfigLogger) logProtocolLimits(p map[protocol.ID]rcmgr.ResourceLimits) {
	for pName, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("protocol_%s", pName), pLimits)
		lg.Info().Msg("protocol limits set")
	}
}

func (l *limitConfigLogger) logPeerLimits(p map[peer.ID]rcmgr.ResourceLimits) {
	for pId, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("peer_%s", p2plogging.PeerId(pId)), pLimits)
		lg.Info().Msg("peer limits set")
	}
}

func (l *limitConfigLogger) logPeerProtocolLimits(p map[protocol.ID]rcmgr.ResourceLimits) {
	for pName, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("protocol_peer_%s", pName), pLimits)
		lg.Info().Msg("protocol peer limits set")
	}
}

// ApplyInboundStreamLimits applies the inbound stream limits to the concrete limit config. The concrete limit config is assumed coming from scaling the
// base limit config by the scaling factor. The inbound stream limits are applied to the concrete limit config if the concrete limit config is greater than
// the inbound stream limits.
// The inbound limits are assumed coming from the config file.
// Args:
//
//	logger: the logger to log the applied limits.
//	concrete: the concrete limit config.
//	limit: the inbound stream limits.
//
// Returns:
//
//	a copy of the concrete limit config with the inbound stream limits applied and overridden.
func ApplyInboundStreamLimits(logger zerolog.Logger, concrete rcmgr.ConcreteLimitConfig, limit p2pconf.InboundStreamLimit) rcmgr.ConcreteLimitConfig {
	c := concrete.ToPartialLimitConfig()

	partial := rcmgr.PartialLimitConfig{}
	lg := logger.With().Logger()

	if int(c.System.StreamsInbound) > limit.System {
		lg = lg.With().Int("concrete_system_inbound_streams", int(c.System.StreamsInbound)).Int("partial_system_inbound_streams", limit.System).Logger()
		partial.System.StreamsInbound = rcmgr.LimitVal(limit.System)
	}

	if int(c.Transient.StreamsInbound) > limit.Transient {
		lg = lg.With().Int("concrete_transient_inbound_streams", int(c.Transient.StreamsInbound)).Int("partial_transient_inbound_streams", limit.Transient).Logger()
		partial.Transient.StreamsInbound = rcmgr.LimitVal(limit.Transient)
	}

	if int(c.ProtocolDefault.StreamsInbound) > limit.Protocol {
		lg = lg.With().Int("concrete_protocol_default_inbound_streams", int(c.ProtocolDefault.StreamsInbound)).Int("partial_protocol_default_inbound_streams",
			limit.Protocol).Logger()
		partial.ProtocolDefault.StreamsInbound = rcmgr.LimitVal(limit.Protocol)
	}

	if int(c.PeerDefault.StreamsInbound) > limit.Peer {
		lg = lg.With().Int("concrete_peer_default_inbound_streams", int(c.PeerDefault.StreamsInbound)).Int("partial_peer_default_inbound_streams", limit.Peer).Logger()
		partial.PeerDefault.StreamsInbound = rcmgr.LimitVal(limit.Peer)
	}

	if int(c.ProtocolPeerDefault.StreamsInbound) > limit.ProtocolPeer {
		lg = lg.With().Int("concrete_protocol_peer_default_inbound_streams",
			int(c.ProtocolPeerDefault.StreamsInbound)).Int("partial_protocol_peer_default_inbound_streams", limit.ProtocolPeer).Logger()
		partial.ProtocolPeerDefault.StreamsInbound = rcmgr.LimitVal(limit.ProtocolPeer)
	}

	if int(c.Stream.StreamsInbound) > limit.Peer {
		lg = lg.With().Int("concrete_stream_inbound_streams", int(c.Stream.StreamsInbound)).Int("partial_stream_inbound_streams", limit.Peer).Logger()
		partial.ServiceDefault.StreamsInbound = rcmgr.LimitVal(limit.Peer)
	}

	if int(c.Conn.StreamsInbound) > limit.Peer {
		lg = lg.With().Int("concrete_conn_inbound_streams", int(c.Conn.StreamsInbound)).Int("partial_conn_inbound_streams", limit.Peer).Logger()
		partial.ServicePeerDefault.StreamsInbound = rcmgr.LimitVal(limit.Peer)
	}

	lg.Info().Msg("inbound stream limits applied")
	return partial.Build(concrete)
}

// ApplyInboundConnectionLimits applies the inbound connection limits to the concrete limit config. The concrete limit config is assumed coming from scaling the
// base limit config by the scaling factor. The inbound connection limits are applied to the concrete limit config if the concrete limit config is greater than
// the inbound connection limits.
// The inbound limits are assumed coming from the config file.
// Args:
//
//	logger: the logger to log the applied limits.
//	concrete: the concrete limit config.
//	peerLimit: the inbound connection limit from each remote peer.
//
// Returns:
//
//	a copy of the concrete limit config with the inbound connection limits applied and overridden.
func ApplyInboundConnectionLimits(logger zerolog.Logger, concrete rcmgr.ConcreteLimitConfig, peerLimit int) rcmgr.ConcreteLimitConfig {
	c := concrete.ToPartialLimitConfig()

	partial := rcmgr.PartialLimitConfig{}
	lg := logger.With().Logger()

	if int(c.PeerDefault.ConnsInbound) > peerLimit {
		lg = lg.With().Int("concrete_peer_inbound_conns", int(c.PeerDefault.ConnsInbound)).Int("partial_peer_inbound_conns", peerLimit).Logger()
		partial.PeerDefault.ConnsInbound = rcmgr.LimitVal(peerLimit)
	}

	lg.Info().Msg("inbound connection limits applied")
	return partial.Build(concrete)
}
