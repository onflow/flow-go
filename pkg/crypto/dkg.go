package crypto

import (
	"fmt"
)

// DKGtype is the supported DKG type
type DKGtype int

// Supported DKG protocols
const (
	// FeldmanVSS is Feldman Verifiable Secret Sharing
	FeldmanVSS DKGtype = iota
	// Joit Feldman
	JointFeldman
	// Gennaro et.al protocl
	GJKR
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
	PrivateKey() PrKey
	// StartDKG starts running a DKG
	StartDKG() (*DKGoutput, error)
	// EndDKG ends a DKG protocol, the public data and node private key are computed
	EndDKG() error
}

func NewDKG(dkg DKGtype, size int, currentIndex int, leaderIndex int) (DKGstate, error) {
	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	// assuming the protocol requires:
	//   m<=t for unforgeability
	//   n-m<=t+1 for robustness
	threshold := (size - 1) / 2
	if dkg == FeldmanVSS {
		common := &(DKGcommon{
			size:         size,
			threshold:    threshold,
			currentIndex: currentIndex,
		})
		fvss := &(feldmanVSSstate{
			DKGcommon:   common,
			leaderIndex: leaderIndex,
		})
		fvss.init()
		return fvss, nil
	}

	return nil, cryptoError{fmt.Sprintf("The Distributed Key Generation %d is not supported.", dkg)}
}

type DKGcommon struct {
	size         int
	threshold    int
	currentIndex int
	running      bool
}

// Size returns the size of the DKG group n
func (s *DKGcommon) Size() int {
	return s.size
}

// Threshold returns the threshold value t
func (s *DKGcommon) Threshold() int {
	return s.threshold
}
