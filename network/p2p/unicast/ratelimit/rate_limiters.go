package ratelimit

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
)

const (
	cleanUpTickInterval = 10 * time.Minute
	rateLimiterTTL      = 10 * time.Minute
)

var (
	MessageCount RateLimitReason = "messagecount"
	Bandwidth    RateLimitReason = "bandwidth"
)

type RateLimitReason string

func (r RateLimitReason) String() string {
	return string(r)
}

type OnRateLimitedPeerFunc func(pid peer.ID, role, msgType string, topic channels.Topic, reason RateLimitReason) // the callback called each time a peer is rate limited

type RateLimitersOption func(*RateLimiters)

func WithDisabledRateLimiting(disabled bool) RateLimitersOption {
	return func(r *RateLimiters) {
		r.disabled = disabled
	}
}

// RateLimiters used to manage stream and bandwidth rate limiters
type RateLimiters struct {
	MessageRateLimiter   p2p.RateLimiter
	BandWidthRateLimiter p2p.RateLimiter
	OnRateLimitedPeer    OnRateLimitedPeerFunc // the callback called each time a peer is rate limited
	disabled             bool                  // flag allows rate limiter to collect metrics without rate limiting if set to false
}

// NewRateLimiters returns *RateLimiters
func NewRateLimiters(messageLimiter, bandwidthLimiter p2p.RateLimiter, onRateLimitedPeer OnRateLimitedPeerFunc, opts ...RateLimitersOption) *RateLimiters {
	r := &RateLimiters{
		MessageRateLimiter:   messageLimiter,
		BandWidthRateLimiter: bandwidthLimiter,
		OnRateLimitedPeer:    onRateLimitedPeer,
		disabled:             true,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// MessageAllowed will return result from MessageRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) MessageAllowed(peerID peer.ID) bool {
	if r.MessageRateLimiter == nil {
		return true
	}

	if !r.MessageRateLimiter.Allow(peerID, nil) {
		r.onRateLimitedPeer(peerID, "", "", "", MessageCount)

		// avoid rate limiting during dry run
		return r.disabled
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

		// avoid rate limiting during dry runs if disabled set to false
		return r.disabled
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
	if r.MessageRateLimiter != nil {
		go r.MessageRateLimiter.Start()
	}

	if r.BandWidthRateLimiter != nil {
		go r.BandWidthRateLimiter.Start()
	}
}

// Stop stops all limiters.
func (r *RateLimiters) Stop() {
	if r.MessageRateLimiter != nil {
		r.MessageRateLimiter.Stop()
	}

	if r.BandWidthRateLimiter != nil {
		r.BandWidthRateLimiter.Stop()
	}
}
