package messages

type DKGPhase int

const (
	DKGPhase1 DKGPhase = iota + 1
	DKGPhase2
	DKGPhase3
)

type DKGMessage struct {
	Orig         int
	Data         []byte
	EpochCounter uint64
	Phase        DKGPhase
}

func NewDKGMessage(orig int, data []byte, epoch uint64, phase DKGPhase) DKGMessage {
	return DKGMessage{
		Orig:         orig,
		Data:         data,
		EpochCounter: epoch,
		Phase:        phase,
	}
}
