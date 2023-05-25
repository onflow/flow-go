package middleware

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
	// IsDisallowed returns true if the given peer is disallow-listed for any cause.
	// Args:
	// - peerID: the peer to check.
	// Returns:
	// - a list of causes for which the peer is disallow-listed.
	// - true if the peer is disallow-listed for any cause.
	// - false if the peer is not disallow-listed for any cause.
	IsDisallowed(peerID string) ([]DisallowListedCause, bool)

	// DisallowFor disallow-lists the given peer for the given cause.
	// If the peer is already disallow-listed for the given cause, this method is a no-op.
	// Args:
	// - peerID: the peer to disallow-list.
	// - cause: the cause for disallow-listing.
	// Returns:
	// - nil if the peer was successfully disallow-listed.
	// - an error if the peer could not be disallow-listed in the cache, the error is irrecoverable and the current node should
	// crash.
	DisallowFor(peerID string, cause DisallowListedCause) error

	// AllowFor allow-lists the given peer for the given cause.
	// If the peer is not disallow-listed for the given cause, this method is a no-op.
	// Args:
	// - peerID: the peer to allow-list.
	// - cause: the cause for allow-listing.
	// Returns:
	// - nil if the peer was successfully allow-listed.
	// - an error if the peer could not be allow-listed in the cache, the error is irrecoverable and the current node should
	AllowFor(peerID string, cause DisallowListedCause) error
}
