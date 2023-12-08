package p2pbuilder

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
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

// newLimitConfigLogger creates a new limitConfigLogger.
func newLimitConfigLogger(logger zerolog.Logger) *limitConfigLogger {
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
