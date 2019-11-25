package storage

import "github.com/dapperlabs/flow-go/proto/gossip/messages"

// MessageStore is an on memory storage interface that keeps a copy of the
// messages that a node receives. This is done as a support to the hash
// proposals, i.e., to avoid duplicate messages, nodes may send the hash first,
// and seek a confirmation from the receiver on the entire content of the message.
// Although the interface may remain the same, the implementation is subject to
// change especially if a storage is provided by the application layer.
type MessageStore interface {

	// Get receives a hash of a message and returns the corresponding message
	// from storage if it exists
	Get(string) (*messages.GossipMessage, error)

	// Put takes a message and its hash and stores the message with its hash
	// as a key
	Put(string, *messages.GossipMessage) error
}
