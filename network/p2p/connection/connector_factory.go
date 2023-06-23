package connection

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

const (
	// minBackoff is the minimum backoff duration for the backoff connector.
	// We set it to 1 second as we want to let the LibP2PNode be in charge of connection establishment and can disconnect
	// and reconnect to peers as soon as it needs. This is essential to ensure that the allow-listing and disallow-listing
	// time intervals are working as expected.
	minBackoff = 1 * time.Second
	// maxBackoff is the maximum backoff duration for the backoff connector. When the backoff duration reaches this value,
	// it will not increase any further.
	maxBackoff = time.Hour
	// timeUnit is the time unit for the backoff duration. The backoff duration will be a multiple of this value.
	// As we use an exponential backoff, the backoff duration will be a multiple of this value multiplied by the exponential
	// base raised to the exponential offset.
	timeUnit = time.Second
	// exponentialBackOffBase is the base for the exponential backoff. The backoff duration will be a multiple of the time unit
	// multiplied by the exponential base raised to the exponential offset, i.e., exponentialBase^(timeUnit*attempt).
	exponentialBackOffBase = 2.0
	// exponentialBackOffOffset is the offset for the exponential backoff. It acts as a constant that is added result
	// of the exponential base raised to the exponential offset, i.e., exponentialBase^(timeUnit*attempt) + exponentialBackOffOffset.
	// This is used to ensure that the backoff duration is always greater than the time unit. We set this to 0 as we want the
	// backoff duration to be a multiple of the time unit.
	exponentialBackOffOffset = 0
)

// DefaultLibp2pBackoffConnectorFactory is a factory function to create a new BackoffConnector. It uses the default
// values for the backoff connector.
// (https://github.com/libp2p/go-libp2p-pubsub/blob/master/discovery.go#L34)
func DefaultLibp2pBackoffConnectorFactory(host host.Host) func() (*discoveryBackoff.BackoffConnector, error) {
	return func() (*discoveryBackoff.BackoffConnector, error) {
		rngSrc := rand.NewSource(rand.Int63())

		cacheSize := 100
		dialTimeout := time.Minute * 2
		backoff := discoveryBackoff.NewExponentialBackoff(
			minBackoff,
			maxBackoff,
			discoveryBackoff.FullJitter,
			timeUnit,
			exponentialBackOffBase,
			exponentialBackOffOffset,
			rngSrc,
		)
		backoffConnector, err := discoveryBackoff.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
		if err != nil {
			return nil, fmt.Errorf("failed to create backoff connector: %w", err)
		}
		return backoffConnector, nil
	}
}
