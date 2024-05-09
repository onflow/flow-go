package ratelimit

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
)

var (
	ReasonMessageCount RateLimitReason = "messagecount"
	ReasonBandwidth    RateLimitReason = "bandwidth"
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

func WithMessageRateLimiter(messageLimiter p2p.RateLimiter) RateLimitersOption {
	return func(r *RateLimiters) {
		r.MessageRateLimiter = messageLimiter
	}
}

func WithBandwidthRateLimiter(bandwidthLimiter p2p.RateLimiter) RateLimitersOption {
	return func(r *RateLimiters) {
		r.BandWidthRateLimiter = bandwidthLimiter
	}
}

func WithNotifier(notifier p2p.RateLimiterConsumer) RateLimitersOption {
	return func(r *RateLimiters) {
		r.notifier = notifier
	}
}

// RateLimiters used to manage stream and bandwidth rate limiters
type RateLimiters struct {
	MessageRateLimiter   p2p.RateLimiter
	BandWidthRateLimiter p2p.RateLimiter
	notifier             p2p.RateLimiterConsumer
	disabled             bool // flag allows rate limiter to collect metrics without rate limiting if set to false
}

// NewRateLimiters returns *RateLimiters
func NewRateLimiters(opts ...RateLimitersOption) *RateLimiters {
	r := NoopRateLimiters()

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Limiters returns list of all underlying rate limiters.
func (r *RateLimiters) Limiters() []p2p.RateLimiter {
	return []p2p.RateLimiter{r.MessageRateLimiter, r.BandWidthRateLimiter}
}

// MessageAllowed will return result from MessageRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) MessageAllowed(peerID peer.ID) bool {
	if r.MessageRateLimiter == nil {
		return true
	}

	if !r.MessageRateLimiter.Allow(peerID, 0) { // 0 is not used for message rate limiter. It is only used for bandwidth rate limiter.
		r.notifier.OnRateLimitedPeer(peerID, "", "", "", ReasonMessageCount.String())

		// avoid rate limiting during dry run
		return r.disabled
	}

	return true
}

// BandwidthAllowed will return result from BandWidthRateLimiter.Allow. It will invoke the OnRateLimitedPeerFunc
// callback each time a peer is not allowed.
func (r *RateLimiters) BandwidthAllowed(peerID peer.ID, originRole string, msgSize int, msgType string, msgTopic channels.Topic) bool {
	if r.BandWidthRateLimiter == nil {
		return true
	}

	if !r.BandWidthRateLimiter.Allow(peerID, msgSize) {
		r.notifier.OnRateLimitedPeer(peerID, originRole, msgType, msgTopic.String(), ReasonBandwidth.String())

		// avoid rate limiting during dry runs if disabled set to false
		return r.disabled
	}

	return true
}
