package flow

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/storage/merkle"
)

// List of built-in event types.
const (
	EventAccountCreated EventType = "flow.AccountCreated"
	EventAccountUpdated EventType = "flow.AccountUpdated"
)

type EventType string

type Event struct {
	// Type is the qualified event type.
	Type EventType
	// TransactionID is the ID of the transaction this event was emitted from.
	TransactionID Identifier
	// TransactionIndex defines the index of the transaction this event was emitted from within the block.
	// The first transaction has index 0, the second has index 1, and so on.
	TransactionIndex uint32
	// EventIndex defines the ordering of events in a transaction.
	// The first event emitted has index 0, the second has index 1, and so on.
	EventIndex uint32
	// Payload contains the encoded event data.
	Payload []byte
}

// String returns the string representation of this event.
func (e Event) String() string {
	return fmt.Sprintf("%s: %s", e.Type, e.ID())
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (e Event) ID() Identifier {
	return MakeID(e)
}

func (e Event) Checksum() Identifier {
	return MakeID(e)
}

// byteSize returns the number of bytes needed to store the wrapped version of the event.
// returned int is an approximate measure, ignoring the number of bytes needed as headers.
func (e Event) byteSize() int {
	return IdentifierLen + // txID
		4 + // Index
		len(e.Type) + // Type
		4 + // TransactionIndex
		len(e.Payload) // Payload
}

// BlockEvents contains events emitted in a single block.
type BlockEvents struct {
	BlockID        Identifier
	BlockHeight    uint64
	BlockTimestamp time.Time
	Events         []Event
}

type EventsList []Event

// ByteSize returns an approximate number of bytes needed to store the wrapped version of the event.
func (el EventsList) ByteSize() int {
	size := 0
	for _, event := range el {
		size += event.byteSize()
	}
	return size
}

// EventsMerkleRootHash calculates the root hash of events inserted into a
// merkle trie with the hash of event as the key and encoded event as value
func EventsMerkleRootHash(el EventsList) (Identifier, error) {
	tree, err := merkle.NewTree(IdentifierLen)
	if err != nil {
		return ZeroID, fmt.Errorf("instantiating payload trie for key length of %d bytes failed: %w", IdentifierLen, err)
	}

	for _, event := range el {
		// event fingerprint is the rlp encoding of the event
		// eventID is the standard sha3 hash of the event fingerprint
		fingerPrint := fingerprint.Fingerprint(event)
		// computing entityID from the fingerprint
		eventID := MakeIDFromFingerPrint(fingerPrint)
		_, err = tree.Put(eventID[:], fingerPrint)
		if err != nil {
			return ZeroID, err
		}
	}

	var root Identifier
	copy(root[:], tree.Hash())
	return root, nil
}
