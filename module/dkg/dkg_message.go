package dkg

import "github.com/onflow/flow-go/model/flow"

type DKGMessage struct {
	Orig    int
	Data    []byte
	EpochID flow.Identifier
}

func NewDKGMessage(orig int, data []byte, epoch flow.Identifier) DKGMessage {
	return DKGMessage{
		Orig:    orig,
		Data:    data,
		EpochID: epoch,
	}
}
