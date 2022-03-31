package messages

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// DKGMessage is the type of message exchanged between DKG nodes.
type DKGMessage struct {
	Data          []byte
	DKGInstanceID string
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
	OriginID flow.Identifier
}

// PrivDKGMessageOut is a wrapper around a DKGMessage containing the network ID of
// the destination.
type PrivDKGMessageOut struct {
	DKGMessage
	DestID flow.Identifier
}

// BroadcastDKGMessage is a wrapper around a DKGMessage intended for broadcasting.
// It contains a signature of the DKGMessage signed with the staking key of the
// sender. DTTF that BroadcastDKGMessage are communicated via the DKG smart contract
// we must also include the Orig or DKG committee index of the sender so that when these
// messages are consumed we can verify if BroadcastDKGMessage was sent by a DKG committee member.
type BroadcastDKGMessage struct {
	DKGMessage
	Orig      uint64
	Signature crypto.Signature
}

// PrivateDKGMessage is a wrapper around DKGMessage intended for use when communicating
// incoming private DKG messages from the messaging engine. This wrapper adds Orig or DKG committee index of the sender
// which is needed when processing the DKG message in the crypto library.
// PrivDKGMessageIn component flow:		message_engine -> broker -> controller
// At the point where the private message reaches the broker, the broker will get the DKG committee index of the sender
// validate and attach it to an instance of PrivateDKGMessage that will then be forwarded to the controller.
type PrivateDKGMessage struct {
	DKGMessage
	Orig uint64
}
