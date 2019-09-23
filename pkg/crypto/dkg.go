package crypto

import (
	"fmt"

	log "github.com/sirupsen/logrus"
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

type DKGstate interface {
	// Size returns the size of the DKG group n
	Size() int
	// Threshold returns the threshold value t
	Threshold() int
	// StartDKG starts running a DKG
	StartDKG() (*DKGoutput, error)
	// EndDKG ends a DKG protocol, the public data and node private key are finalized
	EndDKG() (PrKey, PubKey, []PubKey, error)
	// ProcessDKGmsg processes a new DKG message received by the current node
	ProcessDKGmsg(int, DKGmsg) *DKGoutput
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
		log.Debug(fmt.Sprintf("new dkg my index %d, leader is %d\n", fvss.currentIndex, fvss.leaderIndex))
		return fvss, nil
	}

	return nil, cryptoError{fmt.Sprintf("The Distributed Key Generation %d is not supported.", dkg)}
}

type DKGcommon struct {
	size         int
	threshold    int
	currentIndex int
	// running is true when the DKG protocol is runnig, is false otherwise
	isrunning bool
}

// Size returns the size of the DKG group n
func (s *DKGcommon) Size() int {
	return s.size
}

// Threshold returns the threshold value t
func (s *DKGcommon) Threshold() int {
	return s.threshold
}

type dkgMsgTag byte

const (
	FeldmanVSSshare dkgMsgTag = iota
	FeldmanVSSVerifVec
)

type DKGToSend struct {
	broadcast bool   // true if it's a broadcasted message, false otherwise
	dest      int    // if boradcast is false, dest is the destination index
	data      DKGmsg // data to be sent, including a tag
}

type DKGmsg []byte

type dkgResult int

const (
	nonApplicable dkgResult = iota
	valid
	invalid
)

type DKGoutput struct {
	result dkgResult
	action []DKGToSend
	err    error
}

// maps the interval [1..c-1,c+1..n] into [1..n-1]
func indexOrder(current int, loop int) int {
	if loop < current {
		return loop
	}
	return loop - 1
}
