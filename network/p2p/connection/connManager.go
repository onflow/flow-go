package connection

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	libp2pconnmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p/connection/internal"
)

const (
	// defaultHighWatermark is the default value for the high watermark (i.e., max number of connections).
	// We assume a complete topology graph with maximum of 500 nodes.
	defaultHighWatermark = 500

	// defaultLowWatermark is the default value for the low watermark (i.e., min number of connections).
	// We assume a complete topology graph with minimum of 450 nodes.
	defaultLowWatermark = 450

	// defaultGracePeriod is the default value for the grace period (i.e., time to wait before pruning a new connection).
	defaultGracePeriod = 1 * time.Minute

	// defaultSilencePeriod is the default value for the silence period (i.e., time to wait before start pruning connections).
	defaultSilencePeriod = 10 * time.Second
)

// DefaultConnManagerConfig returns the default configuration for the connection manager.
func DefaultConnManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		HighWatermark: defaultHighWatermark,
		LowWatermark:  defaultLowWatermark,
		GracePeriod:   defaultGracePeriod,
		SilencePeriod: defaultSilencePeriod,
	}
}

// ConnManager provides an implementation of Libp2p's ConnManager interface (https://godoc.org/github.com/libp2p/go-libp2p-core/connmgr#ConnManager)
// It is called back by libp2p when certain events occur such as opening/closing a stream, opening/closing connection etc.
// This implementation updates networking metrics when a peer connection is added or removed
type ConnManager struct {
	basicConnMgr *libp2pconnmgr.BasicConnMgr
	basicNotifee network.Notifiee
	n            network.Notifiee // the notifiee callback provided by libp2p
	log          zerolog.Logger   // logger to log connection, stream and other statistics about libp2p
	metrics      module.LibP2PConnectionMetrics
}

var _ connmgr.ConnManager = (*ConnManager)(nil)

type ManagerConfig struct {
	// HighWatermark and LowWatermark govern the number of connections are maintained by the ConnManager.
	// When the peer count exceeds the HighWatermark, as many peers will be pruned (and
	// their connections terminated) until LowWatermark peers remain.
	HighWatermark int // naming from libp2p
	LowWatermark  int // naming from libp2p

	// SilencePeriod is the time to wait before start pruning connections.
	SilencePeriod time.Duration // naming from libp2p
	// GracePeriod is the time to wait before pruning a new connection.
	GracePeriod time.Duration // naming from libp2p
}

// NewConnManager creates a new connection manager.
// It errors if creating the basic connection manager of libp2p fails.
// The error is not benign, and we should crash the node if it happens.
// It is a malpractice to start the node without connection manager.
func NewConnManager(logger zerolog.Logger, metric module.LibP2PConnectionMetrics, cfg *ManagerConfig) (*ConnManager, error) {
	basic, err := libp2pconnmgr.NewConnManager(
		cfg.LowWatermark,
		cfg.HighWatermark,
		libp2pconnmgr.WithGracePeriod(cfg.GracePeriod),
		libp2pconnmgr.WithSilencePeriod(cfg.SilencePeriod))
	if err != nil {
		return nil, fmt.Errorf("failed to create basic connection manager of libp2p: %w", err)
	}

	cn := &ConnManager{
		log:          logger.With().Str("component", "connection_manager").Logger(),
		metrics:      metric,
		basicConnMgr: basic,
	}

	// aggregates the notifiee callbacks from libp2p and our own notifiee into one.
	cn.n = internal.NewRelayNotifee(internal.NewLoggerNotifiee(cn.log, metric), cn.basicConnMgr.Notifee())

	cn.log.Info().
		Int("low_watermark", cfg.LowWatermark).
		Int("high_watermark", cfg.HighWatermark).
		Dur("grace_period", cfg.GracePeriod).
		Dur("silence_period", cfg.SilencePeriod).
		Msg("connection manager initialized")

	return cn, nil
}

func (cm *ConnManager) Notifee() network.Notifiee {
	return cm.n
}

func (cm *ConnManager) Protect(id peer.ID, tag string) {
	cm.basicConnMgr.Protect(id, tag)
}

func (cm *ConnManager) Unprotect(id peer.ID, tag string) bool {
	return cm.basicConnMgr.Unprotect(id, tag)
}

func (cm *ConnManager) IsProtected(id peer.ID, tag string) bool {
	return cm.basicConnMgr.IsProtected(id, tag)
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
