package unicast

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
)

// RateLimiters used to manage stream and bandwidth rate limiters
type RateLimiters struct {
	StreamRateLimiter     RateLimiter
	BandWidthRateLimiter  RateLimiter
	OnRateLimitedPeerFunc func(peerID peer.ID) // the callback called each time a peer is rate limited
}

func NewRateLimiters(streamLimiter, bandwidthLimiter RateLimiter, onRateLimitedPeer func(peer.ID)) *RateLimiters {
	return &RateLimiters{
		StreamRateLimiter:     streamLimiter,
		BandWidthRateLimiter:  bandwidthLimiter,
		OnRateLimitedPeerFunc: onRateLimitedPeer,
	}
}

// StreamAllowed will return result from StreamRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) StreamAllowed(peerID peer.ID) bool {
	if r.StreamRateLimiter == nil {
		return true
	}

	if !r.StreamRateLimiter.Allow(peerID, nil) {
		r.OnRateLimitedPeerFunc(peerID)
		return false
	}

	return true
}

// BandwidthAllowed will return result from BandWidthRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) BandwidthAllowed(peerID peer.ID, message *message.Message) bool {
	if r.BandWidthRateLimiter == nil {
		return true
	}

	if !r.BandWidthRateLimiter.Allow(peerID, message) {
		r.OnRateLimitedPeerFunc(peerID)
		return false
	}

	return true
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
		go r.StreamRateLimiter.Stop()
	}

	if r.BandWidthRateLimiter != nil {
		go r.BandWidthRateLimiter.Stop()
	}
}
