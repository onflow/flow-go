package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
)

const keyResourceManagerLimit = "libp2p_resource_manager_limit"

// notEjectedPeerFilter returns a PeerFilter that will return an error if the peer is unknown or ejected.
func notEjectedPeerFilter(idProvider module.IdentityProvider) p2p.PeerFilter {
	return func(p peer.ID) error {
		if id, found := idProvider.ByPeerID(p); !found {
			return fmt.Errorf("failed to get identity of unknown peer with peer id %s", p.String())
		} else if id.Ejected {
			return fmt.Errorf("peer %s with node id %s is ejected", p.String(), id.NodeID.String())
		}

		return nil
	}
}

type limitConfigLogger struct {
	logger zerolog.Logger
}

// newLimitConfigLogger creates a new limitConfigLogger.
func newLimitConfigLogger(logger zerolog.Logger) *limitConfigLogger {
	return &limitConfigLogger{logger: logger}
}

// withBaseLimit appends the base limit to the logger with the given prefix.
func (l *limitConfigLogger) withBaseLimit(prefix string, baseLimit rcmgr.BaseLimit) zerolog.Logger {
	return l.logger.With().
		Str("key", keyResourceManagerLimit).
		Int(fmt.Sprintf("%s_streams", prefix), baseLimit.Streams).
		Int(fmt.Sprintf("%s_streams_inbound", prefix), baseLimit.StreamsInbound).
		Int(fmt.Sprintf("%s_streams_outbound", prefix), baseLimit.StreamsOutbound).
		Int(fmt.Sprintf("%s_conns", prefix), baseLimit.Conns).
		Int(fmt.Sprintf("%s_conns_inbound", prefix), baseLimit.ConnsInbound).
		Int(fmt.Sprintf("%s_conns_outbound", prefix), baseLimit.ConnsOutbound).
		Int(fmt.Sprintf("%s_file_descriptors", prefix), baseLimit.FD).
		Int64(fmt.Sprintf("%s_memory", prefix), baseLimit.Memory).Logger()
}

func (l *limitConfigLogger) logResourceManagerLimits(config rcmgr.LimitConfig) {
	l.logGlobalResourceLimits(config)
	l.logServiceLimits(config.Service)
	l.logProtocolLimits(config.Protocol)
	l.logPeerLimits(config.Peer)
	l.logPeerProtocolLimits(config.ProtocolPeer)
}

func (l *limitConfigLogger) logGlobalResourceLimits(config rcmgr.LimitConfig) {
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

func (l *limitConfigLogger) logServiceLimits(s map[string]rcmgr.BaseLimit) {
	for sName, sLimits := range s {
		lg := l.withBaseLimit(fmt.Sprintf("service_%s", sName), sLimits)
		lg.Info().Msg("service limits set")
	}
}

func (l *limitConfigLogger) logProtocolLimits(p map[protocol.ID]rcmgr.BaseLimit) {
	for pName, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("protocol_%s", pName), pLimits)
		lg.Info().Msg("protocol limits set")
	}
}

func (l *limitConfigLogger) logPeerLimits(p map[peer.ID]rcmgr.BaseLimit) {
	for pId, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("peer_%s", pId.String()), pLimits)
		lg.Info().Msg("peer limits set")
	}
}

func (l *limitConfigLogger) logPeerProtocolLimits(p map[protocol.ID]rcmgr.BaseLimit) {
	for pName, pLimits := range p {
		lg := l.withBaseLimit(fmt.Sprintf("protocol_peer_%s", pName), pLimits)
		lg.Info().Msg("protocol peer limits set")
	}
}
