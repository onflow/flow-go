package messages

import "github.com/onflow/flow-go/model/flow"

type DKGMessage struct {
	Orig          int
	Data          []byte
	DKGInstanceID string
}

func NewDKGMessage(orig int, data []byte, dkgInstanceID string) DKGMessage {
	return DKGMessage{
		Orig:          orig,
		Data:          data,
		DKGInstanceID: dkgInstanceID,
	}
}

type DKGMessageIn struct {
	DKGMessage
	OriginID flow.Identifier
}

type DKGMessageOut struct {
	DKGMessage
	DestID flow.Identifier
}
