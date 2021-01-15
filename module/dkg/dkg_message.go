package dkg

type DKGMessage struct {
	Orig    int
	Data    []byte
	EpochID uint64
}

func NewDKGMessage(orig int, data []byte, epoch uint64) DKGMessage {
	return DKGMessage{
		Orig:    orig,
		Data:    data,
		EpochID: epoch,
	}
}
