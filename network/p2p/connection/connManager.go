package connection

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pconnmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

const (
	// DefaultHighWatermark is the default value for the high watermark (i.e., max number of connections).
	// We assume a complete topology graph with maximum of 500 nodes.
	defaultHighWatermark = 500

	// DefaultLowWatermark is the default value for the low watermark (i.e., min number of connections).
	// We assume a complete topology graph with minimum of 450 nodes.
	defaultLowWatermark = 450
)

// DefaultConnManagerConfig returns the default configuration for the connection manager.
func DefaultConnManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		HighWatermark: defaultHighWatermark,
		LowWatermark:  defaultLowWatermark,
	}
}

// ConnManager provides an implementation of Libp2p's ConnManager interface (https://godoc.org/github.com/libp2p/go-libp2p-core/connmgr#ConnManager)
// It is called back by libp2p when certain events occur such as opening/closing a stream, opening/closing connection etc.
// This implementation updates networking metrics when a peer connection is added or removed
type ConnManager struct {
	basicConnMgr *libp2pconnmgr.BasicConnMgr
	n            network.Notifiee // the notifiee callback provided by libp2p
	log          zerolog.Logger   // logger to log connection, stream and other statistics about libp2p
	metrics      module.LibP2PConnectionMetrics
	mu           sync.RWMutex
	protected    map[peer.ID]map[string]struct{}
}

var _ connmgr.ConnManager = (*ConnManager)(nil)

type ManagerConfig struct {
	// HighWatermark and LowWatermark govern the number of connections are maintained by the ConnManager.
	// When the peer count exceeds the HighWatermark, as many peers will be pruned (and
	// their connections terminated) until LowWatermark peers remain.
	HighWatermark int // naming from libp2p
	LowWatermark  int // naming from libp2p
}

func NewConnManager(logger zerolog.Logger, metric module.LibP2PConnectionMetrics, cfg *ManagerConfig) (*ConnManager, error) {
	basic, err := libp2pconnmgr.NewConnManager(cfg.LowWatermark, cfg.HighWatermark)
	if err != nil {
		return nil, fmt.Errorf("failed to create basic connection manager of libp2p: %w", err)
	}

	cn := &ConnManager{
		log:          logger.With().Str("component", "connection_manager").Logger(),
		metrics:      metric,
		protected:    make(map[peer.ID]map[string]struct{}),
		basicConnMgr: basic,
	}
	n := &network.NotifyBundle{
		ListenCloseF:  cn.ListenCloseNotifee,
		ListenF:       cn.ListenNotifee,
		ConnectedF:    cn.Connected,
		DisconnectedF: cn.Disconnected,
	}
	cn.n = n

	return cn, nil
}

func (cm *ConnManager) Notifee() network.Notifiee {
	return cm.n
}

// ListenNotifee is called by libp2p when network starts listening on an addr
func (cm *ConnManager) ListenNotifee(_ network.Network, m multiaddr.Multiaddr) {
	cm.log.Debug().Str("multiaddress", m.String()).Msg("listen started")
}

// called by libp2p when network stops listening on an addr
// * This is never called back by libp2p currently and may be a bug on their side
func (cm *ConnManager) ListenCloseNotifee(_ network.Network, m multiaddr.Multiaddr) {
	// just log the multiaddress on which we listen
	cm.log.Debug().Str("multiaddress", m.String()).Msg("listen stopped")
}

// Connected is called by libp2p when a connection opened
func (cm *ConnManager) Connected(n network.Network, con network.Conn) {
	cm.logConnectionUpdate(n, con, "connection established")
	cm.updateConnectionMetric(n)
}

// Disconnected is called by libp2p when a connection closed
func (cm *ConnManager) Disconnected(n network.Network, con network.Conn) {
	cm.logConnectionUpdate(n, con, "connection removed")
	cm.updateConnectionMetric(n)
}

func (cm *ConnManager) updateConnectionMetric(n network.Network) {
	var totalInbound uint = 0
	var totalOutbound uint = 0

	for _, conn := range n.Conns() {
		switch conn.Stat().Direction {
		case network.DirInbound:
			totalInbound++
		case network.DirOutbound:
			totalOutbound++
		}
	}

	cm.metrics.InboundConnections(totalInbound)
	cm.metrics.OutboundConnections(totalOutbound)
}

func (cm *ConnManager) logConnectionUpdate(n network.Network, con network.Conn, logMsg string) {
	cm.log.Debug().
		Str("remote_peer", con.RemotePeer().String()).
		Str("remote_address", con.RemoteMultiaddr().String()).
		Str("local_peer", con.LocalPeer().String()).
		Str("local_address", con.LocalMultiaddr().String()).
		Str("direction", con.Stat().Direction.String()).
		Int("total_connections", len(n.Conns())).
		Msg(logMsg)
}

func (cm *ConnManager) Protect(id peer.ID, tag string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		tags = make(map[string]struct{}, 2)
		cm.protected[id] = tags
	}
	tags[tag] = struct{}{}
}

func (cm *ConnManager) Unprotect(id peer.ID, tag string) (protected bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}
	if delete(tags, tag); len(tags) == 0 {
		delete(cm.protected, id)
		return false
	}
	return true
}

func (cm *ConnManager) IsProtected(id peer.ID, tag string) (protected bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	tags, ok := cm.protected[id]
	if !ok {
		return false
	}

	if tag == "" {
		return true
	}

	_, protected = tags[tag]
	return protected
}

func (cm *ConnManager) TagPeer(id peer.ID, s string, i int) {
	cm.basicConnMgr.TagPeer(id, s, i)
}

func (cm *ConnManager) UntagPeer(p peer.ID, tag string) {
	cm.basicConnMgr.UntagPeer(p, tag)
}

func (cm *ConnManager) UpsertTag(p peer.ID, tag string, upsert func(int) int) {
	cm.basicConnMgr.UpsertTag(p, tag, upsert)
}

func (cm *ConnManager) GetTagInfo(p peer.ID) *connmgr.TagInfo {
	return cm.basicConnMgr.GetTagInfo(p)
}

func (cm *ConnManager) TrimOpenConns(ctx context.Context) {
	cm.basicConnMgr.TrimOpenConns(ctx)
}

func (cm *ConnManager) Close() error {
	return cm.basicConnMgr.Close()
}
