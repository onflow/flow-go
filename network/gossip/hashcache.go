package gossip

// HashCache is an on-memory cache of recent hashes received
// and confirmed, this is for the gossip layer to have a memory of the
// digests of messages that it received, and to help it discard a duplicate
// message at the proposal time, i.e., the time that another node proposes a
// new message to the node by just sending its hash. By comparing the hash
// proposal against the hashes in the cache, the node can make sure that it
// is very likely to receive a new message instead of a duplicate, and hence
// to save its bandwidth and resources. The cache implementation may be
// replaced by a service provided by an upper layer.
// HashCache has two parts:
// Receive: keeps cache of the messages that the node received completely
// Confirm: keeps cache of the messages that the node received their hash proposal, confirmed the proposal, but not yet
// the message itself
type HashCache interface {
	// Receive adds a hash to the received cache. Returns true if it already existed before and false otherwise
	Receive(hash string) bool
	// IsReceived returns true if the hash exists in the received cache and false otherwise
	IsReceived(hash string) bool

	// Confirm adds a hash to the confirmed cache. Returns true if it already existed before and false otherwise
	Confirm(hash string) bool
	// IsConfirmed returns true if the hash exists in the confirmed cache and false otherwise
	IsConfirmed(hash string) bool
}
