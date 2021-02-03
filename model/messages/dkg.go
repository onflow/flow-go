package messages

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
