package unicast

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
)

var (
	StreamCount RateLimitReason = "streamcount"
	Bandwidth   RateLimitReason = "bandwidth"
)

type RateLimitReason string

func (r RateLimitReason) String() string {
	return string(r)
}

type OnRateLimitedPeerFunc func(pid peer.ID, role, msgType string, topic channels.Topic, reason RateLimitReason) // the callback called each time a peer is rate limited

// RateLimiters used to manage stream and bandwidth rate limiters
type RateLimiters struct {
	StreamRateLimiter    RateLimiter
	BandWidthRateLimiter RateLimiter
	OnRateLimitedPeer    OnRateLimitedPeerFunc // the callback called each time a peer is rate limited
	dryRun               bool
}

// NewRateLimiters returns *RateLimiters
func NewRateLimiters(streamLimiter, bandwidthLimiter RateLimiter, onRateLimitedPeer OnRateLimitedPeerFunc, dryRun bool) *RateLimiters {
	return &RateLimiters{
		StreamRateLimiter:    streamLimiter,
		BandWidthRateLimiter: bandwidthLimiter,
		OnRateLimitedPeer:    onRateLimitedPeer,
		dryRun:               dryRun,
	}
}

// NoopRateLimiters returns noop rate limiters.
func NoopRateLimiters() *RateLimiters {
	return &RateLimiters{
		StreamRateLimiter:    nil,
		BandWidthRateLimiter: nil,
		OnRateLimitedPeer:    nil,
		dryRun:               true,
	}
}

// StreamAllowed will return result from StreamRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) StreamAllowed(peerID peer.ID) bool {
	if r.StreamRateLimiter == nil {
		return true
	}

	if !r.StreamRateLimiter.Allow(peerID, nil) {
		r.onRateLimitedPeer(peerID, "", "", "", StreamCount)

		// avoid rate limiting during dry run
		return r.dryRun
	}

	return true
}

// BandwidthAllowed will return result from BandWidthRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) BandwidthAllowed(peerID peer.ID, role string, msg *message.Message) bool {
	if r.BandWidthRateLimiter == nil {
		return true
	}

	if !r.BandWidthRateLimiter.Allow(peerID, msg) {
		r.onRateLimitedPeer(peerID, role, msg.Type, channels.Topic(msg.ChannelID), Bandwidth)

		// avoid rate limiting during dry run
		return r.dryRun
	}

	return true
}

// onRateLimitedPeer invokes the r.onRateLimitedPeer callback if it is not nil
func (r *RateLimiters) onRateLimitedPeer(peerID peer.ID, role, msgType string, topic channels.Topic, reason RateLimitReason) {
	if r.OnRateLimitedPeer != nil {
		r.OnRateLimitedPeer(peerID, role, msgType, topic, reason)
	}
}

// Start starts the cleanup loop for all limiters
func (r *RateLimiters) Start() {
	if r.StreamRateLimiter != nil {
		go r.StreamRateLimiter.Start()
	}

	if r.BandWidthRateLimiter != nil {
		go r.BandWidthRateLimiter.Start()
	}
}

// Stop stops all limiters.
func (r *RateLimiters) Stop() {
	if r.StreamRateLimiter != nil {
		r.StreamRateLimiter.Stop()
	}

	if r.BandWidthRateLimiter != nil {
		r.BandWidthRateLimiter.Stop()
	}
}
