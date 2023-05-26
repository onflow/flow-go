package middleware

import "github.com/libp2p/go-libp2p/core/peer"

// DisallowListedCause is a type representing the cause of disallow listing. A remote node may be disallow-listed by the
// current node for a variety of reasons. This type is used to represent the reason for disallow-listing, so that if
// a node is disallow-listed for reasons X and Y, allow-listing it back for reason X does not automatically allow-list
// it for reason Y.
type DisallowListedCause string

const (
	// DisallowListedCauseAdmin is the cause of disallow-listing a node by an admin command.
	DisallowListedCauseAdmin DisallowListedCause = "disallow-listed-admin"
	// DisallowListedCauseAlsp is the cause of disallow-listing a node by the ALSP (Application Layer Spam Prevention).
	DisallowListedCauseAlsp DisallowListedCause = "disallow-listed-alsp"
)

// DisallowListCache is an interface for a cache that keeps the list of disallow-listed peers.
// It is designed to present a centralized interface for keeping track of disallow-listed peers for different reasons.
type DisallowListCache interface {
	// GetAllDisallowedListCausesFor returns the list of causes for which the given peer is disallow-listed.
	// Args:
	// - peerID: the peer to check.
	// Returns:
	// - the list of causes for which the given peer is disallow-listed. If the peer is not disallow-listed for any reason,
	// an empty list is returned.
	GetAllDisallowedListCausesFor(peerID peer.ID) []DisallowListedCause

	// DisallowFor disallow-lists a peer for a cause.
	// Args:
	// - peerID: the peerID of the peer to be disallow-listed.
	// - cause: the cause for disallow-listing the peer.
	// Returns:
	// - the list of causes for which the peer is disallow-listed.
	// - error if the operation fails, error is irrecoverable.
	DisallowFor(peerID peer.ID, cause DisallowListedCause) ([]DisallowListedCause, error)

	// AllowFor removes a cause from the disallow list cache entity for the peerID.
	// Args:
	// - peerID: the peerID of the peer to be allow-listed.
	// - cause: the cause for allow-listing the peer.
	// Returns:
	// - the list of causes for which the peer is disallow-listed.
	AllowFor(peerID peer.ID, cause DisallowListedCause) []DisallowListedCause
}
