package ratelimit

import (
	"github.com/onflow/flow-go/model/flow"
)

// RateLimiter is an interface for checking if an address is rate limited.
// By convention, the address used is the payer field of a transaction.
// This rate limiter is applied when a transaction is first received by a
// node, meaning that if a transaction is rate-limited it will be dropped.
type RateLimiter interface {
	// IsRateLimited returns true if the address is rate limited
	IsRateLimited(address flow.Address) bool
}

type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter {
	return &NoopLimiter{}
}

func (l *NoopLimiter) IsRateLimited(address flow.Address) bool {
	return false
}
