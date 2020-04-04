// +build relic

package crypto

import (
	"fmt"
)

// DKGType is the supported DKG type
type DKGType int

// Supported DKG protocols
const (
	// FeldmanVSS is Feldman Verifiable Secret Sharing
	FeldmanVSS DKGType = iota
	// FeldmanVSSQual is Feldman Verifiable Secret Sharing using a complaint
	// system to qualify/disqualify the leader
	FeldmanVSSQual
	// Joint Feldman (Pedersen)
	JointFeldman
)

type DKGstate interface {
	// Size returns the size of the DKG group n
	Size() int
	// Threshold returns the threshold value t
	Threshold() int
	// StartDKG starts running a DKG
	StartDKG(seed []byte) error
	// HandleMsg processes a new DKG message received by the current node
	// orig is the message origin index
	HandleMsg(orig int, msg []byte) error
	// EndDKG ends a DKG protocol, the public data and node private key are finalized
	EndDKG() (PrivateKey, PublicKey, []PublicKey, error)
	// NextTimeout set the next timeout of the protocol if any timeout applies
	NextTimeout() error
	// Running returns the running state of the DKG protocol
	Running() bool
	// Disqualify forces a node to get disqualified
	// for a reason outside of the DKG protocol
	// The caller should make sure all honest nodes call this function,
	// otherwise, the protocol can be broken
	Disqualify(node int) error
}

// index is the node index type used as participants ID
type index byte

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m<=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}

// NewDKG creates a new instance of a DKG protocol.
// An instance is run by a single node and is usable for only one protocol.
// In order to run the protocol again, a new instance needs to be created
func NewDKG(dkg DKGType, size int, currentIndex int,
	processor DKGProcessor, leaderIndex int) (DKGstate, error) {
	if size < DKGMinSize || size > DKGMaxSize {
		return nil, fmt.Errorf("size should be between %d and %d", DKGMinSize, DKGMaxSize)
	}

	if currentIndex >= size || leaderIndex >= size || currentIndex < 0 || leaderIndex < 0 {
		return nil, fmt.Errorf("indices of current and leader nodes must be between 0 and %d", size-1)
	}

	// optimal threshold (t) to allow the largest number of malicious nodes (m)
	threshold := optimalThreshold(size)
	common := &dkgCommon{
		size:         size,
		threshold:    threshold,
		currentIndex: index(currentIndex),
		processor:    processor,
	}
	switch dkg {
	case FeldmanVSS:
		fvss := &feldmanVSSstate{
			dkgCommon:   common,
			leaderIndex: index(leaderIndex),
		}
		fvss.init()
		return fvss, nil
	case FeldmanVSSQual:
		fvss := &feldmanVSSstate{
			dkgCommon:   common,
			leaderIndex: index(leaderIndex),
		}
		fvssq := &feldmanVSSQualState{
			feldmanVSSstate: fvss,
			disqualified:    false,
		}
		fvssq.init()
		return fvssq, nil
	case JointFeldman:
		jf := &JointFeldmanState{
			dkgCommon: common,
		}
		jf.init()
		return jf, nil
	default:
		return nil, fmt.Errorf("the Distributed Key Generation %d is not supported", dkg)
	}
}

// dkgCommon holds the common data of all DKG protocols
type dkgCommon struct {
	size         int
	threshold    int
	currentIndex index
	// running is true when the DKG protocol is running, is false otherwise
	running bool
	// processes the action of the DKG interface outputs
	processor DKGProcessor
}

// Running returns the running state of the DKG protocol
// The state is equal to true when the DKG protocol is running, and is equal to false otherwise.
func (s *dkgCommon) Running() bool {
	return s.running
}

// Size returns the size of the DKG group n
func (s *dkgCommon) Size() int {
	return s.size
}

// Threshold returns the threshold value t
func (s *dkgCommon) Threshold() int {
	return s.threshold
}

// NextTimeout sets the next protocol timeout if there is any.
// this function should be overwritten by any protocol that uses timeouts
func (s *dkgCommon) NextTimeout() error {
	return nil
}

// dkgMsgTag is the type used to encode message tags
type dkgMsgTag byte

const (
	feldmanVSSShare dkgMsgTag = iota
	feldmanVSSVerifVec
	feldmanVSSComplaint
	feldmanVSSComplaintAnswer
)

// DKGProcessor is an interface that implements the DKG output actions
// an instance of a DKGactor is needed for each DKG
type DKGProcessor interface {
	// sends a private message to a destination
	Send(dest int, data []byte)
	// broadcasts a message to all dkg nodes
	// This function needs to make sure all nodes have received the same message
	Broadcast(data []byte)
	// flags that a node is misbehaving (deserves slashing)
	Blacklist(node int)
	// flags that a node is misbehaving (but can't be slashed)
	FlagMisbehavior(node int, log string)
}

const (
	wrongFormat   = "wrong message format"
	duplicated    = "message type is duplicated"
	wrongProtocol = "message is not compliant with the protocol"
)
