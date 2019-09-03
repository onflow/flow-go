package crypto

// AlgoName is the supported algos type
type DKGtype int

const (
	// Supported DKG protocols
	FeldmanVSS DKGtype = iota
	JointFeldman
	JGKR
)

type DKGinput struct { // should be protobuff
	// Broadcast or 1-n or 1-1
	// sender
	// message type
	// message data
}

type DKGoutput struct {
	// result: Valid, Invalid, Nil
	// [ (message, [dest...])...]         -- dest : Indexes or ALL
	// err
}

type DKGstate interface {
	// Size returns the size of the DKG group n
	Size() int
	// Threshold returns the threshold value t
	Threshold() int
	// PrivateKey returns the private key share of the current node
	PrivateKey() PrivateKey
	// StartDKG starts running a DKG
	StartDKG() (DKGoutput, Error)
}	


func NewDKG(type DKGtype, size int, myIndex int, leaderIndex int) (DKGstate, Error) {
	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	// assuming the protocol requires:
	//   m<=t for unforgeability
	//   n-m<=t+1 for robustness
	threshold := (size-1)/2
	if type == FeldmanVSS {
		common := &(DKGcommon{size, threshold, myIndex})
		fvss := &(FeldmanVSS{common, leaderIndex, nil})
		fvss.init()
		return fvss, nil
	}

	return nil, cryptoError{Sprintf("The Distributed Key Generation %d is not supported.", type)}
}



type DKGcommon struct {
	size int
	threshold int
	myIndex int
}

func (s *DKGcommon) Size() {
	return s.size
}

func (s *DKGcommon) Threshold() {
	return s.threshold
}