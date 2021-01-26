package messages

type DKGMessage struct {
	Orig         int
	Data         []byte
	EpochCounter uint64
}

func NewDKGMessage(orig int, data []byte, epoch uint64) DKGMessage {
	return DKGMessage{
		Orig:         orig,
		Data:         data,
		EpochCounter: epoch,
	}
}
