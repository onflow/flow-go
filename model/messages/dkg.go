package messages

type DKGPhase int

const (
	DKGPhase1 DKGPhase = iota + 1
	DKGPhase2
	DKGPhase3
)

type DKGMessage struct {
	Orig          int
	Data          []byte
	DKGInstanceID string
	Phase         DKGPhase
}

func NewDKGMessage(orig int, data []byte, dkgInstanceID string, phase DKGPhase) DKGMessage {
	return DKGMessage{
		Orig:          orig,
		Data:          data,
		DKGInstanceID: dkgInstanceID,
		Phase:         phase,
	}
}
