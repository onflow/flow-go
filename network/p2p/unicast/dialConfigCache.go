package unicast

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p/unicast/internal"
)

// DialConfigCache is a thread-safe cache for dial configs. It is used by the unicast service to store
// the dial configs for peers.
type DialConfigCache interface {
	// Get returns the dial config for the given peer id.
	// Returns:
	// - the dial config and true if the config exists, nil and false otherwise. Note that the
	// returned dial config is a deep copy of the cached config, so it is safe to modify it.
	Get(peerID peer.ID) (*internal.DialConfig, bool)

	// Adjust adjusts the dial config for the given peer id using the given adjustFunc.
	// It returns an error if the adjustFunc returns an error.
	// Args:
	// - peerID: the peer id of the dial config.
	// - adjustFunc: the function that adjusts the dial config.
	// Returns:
	//   - error if the adjustFunc returns an error. Any error should be treated as an irrecoverable error and indicates a bug.
	Adjust(peerID peer.ID, adjustFunc internal.DialConfigAdjustFunc) error
}
