package messages

import "github.com/onflow/flow-go/model/flow"

// DKGMessage is the type of message exchanged between DKG nodes.
type DKGMessage struct {
	Orig          int
	Data          []byte
	DKGInstanceID string
}

// NewDKGMessage creates a new DKGMessage.
func NewDKGMessage(orig int, data []byte, dkgInstanceID string) DKGMessage {
	return DKGMessage{
		Orig:          orig,
		Data:          data,
		DKGInstanceID: dkgInstanceID,
	}
}

// DKGMessageIn is a wrapper around a DKGMessage containing the network ID of
// the sender.
type DKGMessageIn struct {
	DKGMessage
	OriginID flow.Identifier
}

// DKGMessageOut is a wrapper around a DKGMessage containing the network ID of
// the destination.
type DKGMessageOut struct {
	DKGMessage
	DestID flow.Identifier
}
