package unicast

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// ConfigCache is a thread-safe cache for dial configs. It is used by the unicast service to store
// the dial configs for peers.
type ConfigCache interface {
	// GetOrInit returns the dial config for the given peer id. If the config does not exist, it creates a new config
	// using the factory function and stores it in the cache.
	// Args:
	// - peerID: the peer id of the dial config.
	// Returns:
	//   - *Config, the dial config for the given peer id.
	//   - error if the factory function returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
	GetWithInit(peerID peer.ID) (*Config, error)

	// Adjust adjusts the dial config for the given peer id using the given adjustFunc.
	// It returns an error if the adjustFunc returns an error.
	// Args:
	// - peerID: the peer id of the dial config.
	// - adjustFunc: the function that adjusts the dial config.
	// Returns:
	//   - error if the adjustFunc returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
	AdjustWithInit(peerID peer.ID, adjustFunc UnicastConfigAdjustFunc) (*Config, error)

	// Size returns the number of dial configs in the cache.
	Size() uint
}
