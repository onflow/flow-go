package messages

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// DKGMessage is the type of message exchanged between DKG nodes.
//
type DKGMessage struct {
	Data          []byte
	DKGInstanceID string
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedDKGMessage DKGMessage

// NewDKGMessage creates a new instance of DKGMessage.
//
func NewDKGMessage(untrusted UntrustedDKGMessage) (*DKGMessage, error) {
	if len(untrusted.Data) == 0 {
		return nil, fmt.Errorf("data must not be empty")
	}
	if untrusted.DKGInstanceID == "" {
		return nil, fmt.Errorf("DKG instance ID must not be empty")
	}
	return &DKGMessage{
		Data:          untrusted.Data,
		DKGInstanceID: untrusted.DKGInstanceID,
	}, nil
}

// NewDKGMessageFromFields creates a new DKGMessage from individual fields.
func NewDKGMessageFromFields(data []byte, dkgInstanceID string) (*DKGMessage, error) {
	return NewDKGMessage(UntrustedDKGMessage{
		Data:          data,
		DKGInstanceID: dkgInstanceID,
	})
}

// PrivDKGMessageIn is a wrapper around a DKGMessage containing the network ID
// of the sender.
type PrivDKGMessageIn struct {
	DKGMessage
	CommitteeMemberIndex uint64 // CommitteeMemberIndex field is set when the message arrives at the Broker
	OriginID             flow.Identifier
}

// PrivDKGMessageOut is a wrapper around a DKGMessage containing the network ID of
// the destination.
type PrivDKGMessageOut struct {
	DKGMessage
	DestID flow.Identifier
}

// BroadcastDKGMessage is a wrapper around a DKGMessage intended for broadcasting.
// It contains a signature of the DKGMessage signed with the staking key of the
// sender. When the DKG contract receives BroadcastDKGMessage' it will attach the
// NodeID of the sender, we then add this field to the BroadcastDKGMessage when reading broadcast messages.
type BroadcastDKGMessage struct {
	DKGMessage
	CommitteeMemberIndex uint64          `json:"-"` // CommitteeMemberIndex field is set when reading broadcast messages using the NodeID to find the index of the sender in the DKG committee
	NodeID               flow.Identifier `json:"-"` // NodeID field is added when reading broadcast messages from the DKG contract, this field is ignored when sending broadcast messages
	Signature            crypto.Signature
}
