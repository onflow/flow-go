package ledger

import "encoding/hex"

// PayloadID captures a unique ID to a payload
type PayloadID []byte

func (p PayloadID) String() string {
	return hex.EncodeToString(p)
}

// Payload is the smallest immutable storable unit in ledger
type Payload interface {
	// TypeVersion returns the version of this payload encoding (zero is reserved for testing)
	TypeVersion() uint8
	// ID returns a unique id for this
	ID() PayloadID
	// Encode encodes the the payload object
	Encode() []byte
}
