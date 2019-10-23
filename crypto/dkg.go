package crypto

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// DKGType is the supported DKG type
type DKGType int

// Supported DKG protocols
const (
	// FeldmanVSS is Feldman Verifiable Secret Sharing
	FeldmanVSS DKGType = iota
	// Joint Feldman (Pedersen)
	JointFeldman
	// Gennaro et.al protocol
	GJKR
)

type DKGstate interface {
	// Size returns the size of the DKG group n
	Size() int
	// Threshold returns the threshold value t
	Threshold() int
	// StartDKG starts running a DKG
	StartDKG(seed []byte) *DKGoutput
	// EndDKG ends a DKG protocol, the public data and node private key are finalized
	EndDKG() (PrivateKey, PublicKey, []PublicKey, error)
	// ProcessDKGmsg processes a new DKG message received by the current node
	ProcessDKGmsg(int, DKGmsg) *DKGoutput
}

// NewDKG creates a new instance of a DKG protocol.
// An instance is run by a single node and is usable for only one protocol.
// In order to run the protocol again, a new instance needs to be created
func NewDKG(dkg DKGType, size int, currentIndex int, leaderIndex int) (DKGstate, error) {
	if currentIndex >= size || leaderIndex >= size {
		return nil, cryptoError{fmt.Sprintf("Indexes of current and leader nodes must be in the correct range.")}
	}
	minSize := 3
	if size < minSize {
		return nil, cryptoError{fmt.Sprintf("Size should be larger than 3.")}
	}
	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	// assuming the protocol requires:
	//   m<=t for unforgeability
	//   n-m<=t+1 for robustness
	threshold := (size - 1) / 2
	if dkg == FeldmanVSS {
		common := &dkgCommon{
			size:         size,
			threshold:    threshold,
			currentIndex: currentIndex,
		}
		fvss := &feldmanVSSstate{
			dkgCommon:   common,
			leaderIndex: leaderIndex,
		}
		fvss.init()
		log.Debugf("new dkg my index %d, leader is %d\n", fvss.currentIndex, fvss.leaderIndex)
		return fvss, nil
	}

	return nil, cryptoError{fmt.Sprintf("The Distributed Key Generation %d is not supported.", dkg)}
}

// dkgCommon holds the common data of all DKG protocols
type dkgCommon struct {
	size         int
	threshold    int
	currentIndex int
	// running is true when the DKG protocol is runnig, is false otherwise
	running bool
}

// Size returns the size of the DKG group n
func (s *dkgCommon) Size() int {
	return s.size
}

// Threshold returns the threshold value t
func (s *dkgCommon) Threshold() int {
	return s.threshold
}

type dkgMsgTag byte

const (
	FeldmanVSSshare dkgMsgTag = iota
	FeldmanVSSVerifVec
)

type DKGToSend struct {
	broadcast bool   // true if it's a broadcasted message, false otherwise
	dest      int    // if broadcast is false, dest is the destination index
	data      DKGmsg // data to be sent, including a tag
}

// DKGmsg is the data sent in any DKG communication
type DKGmsg []byte

// DKGresult is the supported type for the return values
type DKGresult int

const (
	nonApplicable DKGresult = iota
	valid
	invalid
)

// DKGoutput is the type of the uotput of any DKG fucntion
type DKGoutput struct {
	result DKGresult
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
