package unicast

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
)

var (
	ErrStreamRateLimited    = errors.New("rate-limiting peer unicast stream creation dropping message")
	ErrBandwidthRateLimited = errors.New("rate-limiting peer unicast bandwidth limit exceeded dropping message")
)

// RateLimiters used to manage stream and bandwidth rate limiters
type RateLimiters struct {
	StreamRateLimiter     RateLimiter
	BandWidthRateLimiter  RateLimiter
	OnRateLimitedPeerFunc func(peer.ID, error) // the callback called each time a peer is rate limited
	dryRun                bool
}

// NewRateLimiters returns *RateLimiters
func NewRateLimiters(streamLimiter, bandwidthLimiter RateLimiter, onRateLimitedPeer func(peer.ID, error), dryRun bool) *RateLimiters {
	return &RateLimiters{
		StreamRateLimiter:     streamLimiter,
		BandWidthRateLimiter:  bandwidthLimiter,
		OnRateLimitedPeerFunc: onRateLimitedPeer,
		dryRun:                dryRun,
	}
}

// NoopRateLimiters returns noop rate limiters.
func NoopRateLimiters() *RateLimiters {
	return &RateLimiters{
		StreamRateLimiter:     nil,
		BandWidthRateLimiter:  nil,
		OnRateLimitedPeerFunc: nil,
		dryRun:                true,
	}
}

// StreamAllowed will return result from StreamRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) StreamAllowed(peerID peer.ID) bool {
	if r.StreamRateLimiter == nil {
		return true
	}

	if !r.StreamRateLimiter.Allow(peerID, nil) {
		r.OnRateLimitedPeerFunc(peerID, ErrStreamRateLimited)

		// avoid rate limiting during dry run
		if r.dryRun {
			return true
		}

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
		r.OnRateLimitedPeerFunc(peerID, ErrBandwidthRateLimited)

		// avoid rate limiting during dry run
		if r.dryRun {
			return true
		}

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
		r.StreamRateLimiter.Stop()
	}

	if r.BandWidthRateLimiter != nil {
		r.BandWidthRateLimiter.Stop()
	}
}
