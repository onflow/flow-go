package p2p

import "github.com/libp2p/go-libp2p/core/connmgr"

// ConnectionGater the customized interface for the connection gater in the p2p package.
// It acts as a wrapper around the libp2p connmgr.ConnectionGater interface and adds some custom methods.
type ConnectionGater interface {
	connmgr.ConnectionGater

	// SetDisallowListOracle sets the disallow list oracle for the connection gater.
	// If one is set, the oracle is consulted upon every incoming or outgoing connection attempt, and
	// the connection is only allowed if the remote peer is not on the disallow list.
	// In Flow blockchain, it is not optional to dismiss the disallow list oracle, and if one is not set
	// the connection gater will panic.
	// Also, it follows a dependency injection pattern and does not allow to set the disallow list oracle more than once,
	// any subsequent calls to this method will result in a panic.
	// Args:
	// 	oracle: the disallow list oracle to set.
	// Returns:
	// 	none
	// Panics:
	// 	if the disallow list oracle is already set.
	SetDisallowListOracle(oracle DisallowListOracle)
}
