package messages

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// DKGMessage is the type of message exchanged between DKG nodes.
//
//structwrite:immutable
type DKGMessage struct {
	Data          []byte
	DKGInstanceID string
}

// UntrustedDKGMessage is an untrusted input-only representation of a DKGMessage,
// used for construction.
//
// An instance of UntrustedDKGMessage should be validated and converted into
// a trusted DKGMessage using NewDKGMessage constructor.
type UntrustedDKGMessage DKGMessage

// NewDKGMessage creates a new DKGMessage.
//
// Parameters:
//   - untrusted: untrusted DKGMessage to be validated
//
// Returns:
//   - DKGMessage: the newly created instance
//   - error: any error that occurred during creation
//
// Expected Errors:
//   - TODO: add validation errors
func NewDKGMessage(untrusted UntrustedDKGMessage) (DKGMessage, error) {
	// TODO: add validation logic
	return DKGMessage{Data: untrusted.Data, DKGInstanceID: untrusted.DKGInstanceID}, nil
}

// NewDKGMessage creates a new DKGMessage.
func NewDKGMessage(data []byte, dkgInstanceID string) DKGMessage {
	return DKGMessage{
		Data:          data,
		DKGInstanceID: dkgInstanceID,
	}
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
