package unicast

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/unicast/unicastmodel"
)

const (
	// DefaultDailConfigCacheSize is the default size of the dial config cache.
	// It is used if the size is not specified when creating a new dial config cache.
	// We choose the default size to be big enough so that the eviction is rare.
	// Typically, there should be a single entry in the cache for each authorized node.
	DefaultDailConfigCacheSize = 10_000
)

// DialConfigCache is a thread-safe cache for dial configs. It is used by the unicast service to store
// the dial configs for peers.
type DialConfigCache interface {
	// GetOrInit returns the dial config for the given peer id. If the config does not exist, it creates a new config
	// using the factory function and stores it in the cache.
	// Args:
	// - peerID: the peer id of the dial config.
	// Returns:
	//   - *DialConfig, the dial config for the given peer id.
	//   - error if the factory function returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
	GetOrInit(peerID peer.ID) (*unicastmodel.DialConfig, error)

	// Adjust adjusts the dial config for the given peer id using the given adjustFunc.
	// It returns an error if the adjustFunc returns an error.
	// Args:
	// - peerID: the peer id of the dial config.
	// - adjustFunc: the function that adjusts the dial config.
	// Returns:
	//   - error if the adjustFunc returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
	Adjust(peerID peer.ID, adjustFunc unicastmodel.DialConfigAdjustFunc) (*unicastmodel.DialConfig, error)

	// Size returns the number of dial configs in the cache.
	Size() uint
}
