package connection

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	discoveryBackoff "github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

// DefaultLibp2pBackoffConnectorFactory is a factory function to create a new BackoffConnector. It uses the default
// values for the backoff connector.
// (https://github.com/libp2p/go-libp2p-pubsub/blob/master/discovery.go#L34)
func DefaultLibp2pBackoffConnectorFactory(host host.Host) func() (*discoveryBackoff.BackoffConnector, error) {
	return func() (*discoveryBackoff.BackoffConnector, error) {
		rngSrc := rand.NewSource(rand.Int63())
		minBackoff, maxBackoff := time.Second*10, time.Hour
		cacheSize := 100
		dialTimeout := time.Minute * 2
		backoff := discoveryBackoff.NewExponentialBackoff(
			minBackoff,
			maxBackoff,
			discoveryBackoff.FullJitter,
			time.Second,
			5.0,
			0,
			rand.New(rngSrc))
		backoffConnector, err := discoveryBackoff.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
		if err != nil {
			return nil, fmt.Errorf("failed to create backoff connector: %w", err)
		}
		return backoffConnector, nil
	}
}
