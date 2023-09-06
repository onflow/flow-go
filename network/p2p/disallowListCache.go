package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// DisallowListCache is an interface for a cache that keeps the list of disallow-listed peers.
// It is designed to present a centralized interface for keeping track of disallow-listed peers for different reasons.
type DisallowListCache interface {
	// IsDisallowListed determines whether the given peer is disallow-listed for any reason.
	// Args:
	// - peerID: the peer to check.
	// Returns:
	// - []network.DisallowListedCause: the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
	// a nil slice is returned.
	// - bool: true if the peer is disallow-listed for any reason, false otherwise.
	IsDisallowListed(peerID peer.ID) ([]network.DisallowListedCause, bool)

	// DisallowFor disallow-lists a peer for a cause.
	// Args:
	// - peerID: the peerID of the peer to be disallow-listed.
	// - cause: the cause for disallow-listing the peer.
	// Returns:
	// - the list of causes for which the peer is disallow-listed.
	// - error if the operation fails, error is irrecoverable.
	DisallowFor(peerID peer.ID, cause network.DisallowListedCause) ([]network.DisallowListedCause, error)

	// AllowFor removes a cause from the disallow list cache entity for the peerID.
	// Args:
	// - peerID: the peerID of the peer to be allow-listed.
	// - cause: the cause for allow-listing the peer.
	// Returns:
	// - the list of causes for which the peer is disallow-listed. If the peer is not disallow-listed for any reason,
	// an empty list is returned.
	AllowFor(peerID peer.ID, cause network.DisallowListedCause) []network.DisallowListedCause
}

// DisallowListCacheConfig is the configuration for the disallow-list cache.
// The disallow-list cache is used to temporarily disallow-list peers.
type DisallowListCacheConfig struct {
	// MaxSize is the maximum number of peers that can be disallow-listed at any given time.
	// When the cache is full, no further new peers can be disallow-listed.
	// Recommended size is 100 * number of staked nodes.
	MaxSize uint32

	// Metrics is the HeroCache metrics collector to be used for the disallow-list cache.
	Metrics module.HeroCacheMetrics
}
