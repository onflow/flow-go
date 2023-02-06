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
func (l *limitConfigLogger) withBaseLimit(prefix string, baseLimit rcmgr.BaseLimit) {
	l.logger = l.logger.With().
		Int(fmt.Sprintf("%s_streams", prefix), baseLimit.Streams).
		Int(fmt.Sprintf("%s_streams_inbound", prefix), baseLimit.StreamsInbound).
		Int(fmt.Sprintf("%s_streams_outbound", prefix), baseLimit.StreamsOutbound).
		Int(fmt.Sprintf("%s_conns", prefix), baseLimit.Conns).
		Int(fmt.Sprintf("%s_conns_inbound", prefix), baseLimit.ConnsInbound).
		Int(fmt.Sprintf("%s_conns_outbound", prefix), baseLimit.ConnsOutbound).
		Int(fmt.Sprintf("%s_file_descriptors", prefix), baseLimit.FD).
		Int64(fmt.Sprintf("%s_memory", prefix), baseLimit.Memory).Logger()
}

func (l *limitConfigLogger) loggerForLimits(config rcmgr.LimitConfig) zerolog.Logger {
	l.withBaseLimit("system", config.System)
	l.withBaseLimit("transient", config.Transient)
	l.withBaseLimit("allowed_listed_system", config.AllowlistedSystem)
	l.withBaseLimit("allowed_lister_transient", config.AllowlistedTransient)
	l.withBaseLimit("service_default", config.ServiceDefault)
	l.withBaseLimit("service_peer_default", config.ServicePeerDefault)
	l.withBaseLimit("protocol_default", config.ProtocolDefault)
	l.withBaseLimit("protocol_peer_default", config.ProtocolPeerDefault)
	l.withBaseLimit("peer_default", config.PeerDefault)
	l.withBaseLimit("connections", config.Conn)
	l.withBaseLimit("streams", config.Stream)

	l.withServiceLogger(config.Service)
	l.withProtocolLogger(config.Protocol)
	l.withPeerLogger(config.Peer)
	l.withProtocolPeerLogger(config.ProtocolPeer)
	return l.logger
}

func (l *limitConfigLogger) withServiceLogger(s map[string]rcmgr.BaseLimit) {
	for sName, sLimits := range s {
		l.withBaseLimit(fmt.Sprintf("service_%s", sName), sLimits)
	}
}

func (l *limitConfigLogger) withProtocolLogger(p map[protocol.ID]rcmgr.BaseLimit) {
	for pName, pLimits := range p {
		l.withBaseLimit(fmt.Sprintf("protocol_%s", pName), pLimits)
	}
}

func (l *limitConfigLogger) withPeerLogger(p map[peer.ID]rcmgr.BaseLimit) {
	for pId, pLimits := range p {
		l.withBaseLimit(fmt.Sprintf("peer_%s", pId.String()), pLimits)
	}
}

func (l *limitConfigLogger) withProtocolPeerLogger(p map[protocol.ID]rcmgr.BaseLimit) {
	for pName, pLimits := range p {
		l.withBaseLimit(fmt.Sprintf("protocol_peer_%s", pName), pLimits)
	}
}
