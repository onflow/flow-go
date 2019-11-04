// +build relic

package crypto

// Implements Feldman Verifiable Secret Sharing using BLS G2 group.
// a complaint system is added to qualify/disqualify the leader

type feldmanVSSQualState struct {
	// feldmanVSSstate state
	*feldmanVSSstate
	// complaints received against the leader:
	// the key is the origin of the complaint
	// a complaint will be created if a complaint message or an answer was
	// broadcasted, a complaint will only be checked only when both the
	// complaint message and the answer were broadcasted
	complaints map[index]*complaint
	// is the leader disqualified
	disqualified bool
	// Timeout to receive shares and verification vectors
	// - if a share is not received before this timeout a complaint will be formed
	// - if the verification is not received before this timeout,
	// leader is disqualified
	sharesTimeout bool
	// Timeout to receive complaints
	// all complaints received after this timeout are ignored
	complaintsTimeout bool
}

// these data are required to be stored to justify a slashing
type complaint struct {
	received       bool
	answerReceived bool
	answer         scalar
}

func (s *feldmanVSSQualState) init() {
	s.feldmanVSSstate.init()
	s.complaints = make(map[index]*complaint)
}

// sets the next protocol timeout
func (s *feldmanVSSQualState) NextTimeout() *DKGoutput {
	out := &DKGoutput{
		result: valid,
		action: []DKGToSend{},
		err:    nil,
	}

	if !s.running {
		out.err = cryptoError{"dkg protocol is not running"}
		return out
	}
	// if leader is already disqualified, there is nothing to do
	if s.disqualified {
		if s.sharesTimeout {
			s.complaintsTimeout = true
		} else {
			s.sharesTimeout = true
		}
		return out
	}
	if !s.sharesTimeout && !s.complaintsTimeout {
		out.action = s.setSharesTimeout()
		return out
	}
	if s.sharesTimeout && !s.complaintsTimeout {
		s.setComplaintsTimeout()
		return out
	}
	out.err = cryptoError{"next timeout would be to end DKG protocol"}
	return out
}

// EndDKG ends the protocol and returns the keys
// This is also a timeout to receiving all complaints answers
func (s *feldmanVSSQualState) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.running {
		return nil, nil, nil, cryptoError{"dkg protocol is not running"}
	}
	if !s.sharesTimeout || !s.complaintsTimeout {
		return nil, nil, nil,
			cryptoError{"two timeouts should be set before ending dkg"}
	}
	//fmt.Println(s.currentIndex, len(s.complaints), s.disqualified)
	s.running = false
	// check if a complaint has remained without an answer
	// a leader is disqualified if a complaint was never answered
	if !s.disqualified {
		for _, c := range s.complaints {
			if c.received && !c.answerReceived {
				s.disqualified = true
				break
			}
		}
	}
	//fmt.Println(s.currentIndex, len(s.complaints), s.disqualified)

	// If the leader is disqualified, all keys are ignored
	// otherwise, the keys are valid
	if s.disqualified {
		return nil, nil, nil, nil
	}

	// private key of the current node
	x := &PrKeyBLS_BLS12381{
		alg:    s.blsContext, // signer algo
		scalar: s.x,          // the private share
	}
	// Group public key
	Y := &PubKeyBLS_BLS12381{
		point: s.A[0],
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	for i, p := range s.y {
		y[i] = &PubKeyBLS_BLS12381{
			point: p,
		}
	}
	return x, Y, y, nil
}

const (
	complaintSize      = 1
	complainAnswerSize = 1 + PrKeyLenBLS_BLS12381
)

func (s *feldmanVSSQualState) ReceiveDKGMsg(orig int, msg DKGmsg) *DKGoutput {
	out := &DKGoutput{
		result: invalid,
		action: []DKGToSend{},
		err:    nil,
	}

	if !s.running || orig >= s.Size() || len(msg) == 0 {
		out.result = invalid
		return out
	}

	// In case a broadcasted message is received by the origin node,
	// the message is just ignored
	if s.currentIndex == index(orig) {
		out.result = valid
		return out
	}

	// if leader is already disqualified, ignore the message
	if s.disqualified {
		return out
	}

	switch dkgMsgTag(msg[0]) {
	case FeldmanVSSshare:
		out.result, out.action = s.receiveShare(index(orig), msg[1:])
	case FeldmanVSSVerifVec:
		out.result, out.action = s.receiveVerifVector(index(orig), msg[1:])
	case FeldmanVSSComplaint:
		out.result, out.action = s.receiveComplaint(index(orig), msg[1:])
	case FeldmanVSSComplaintAnswer:
		out.result = s.receiveComplaintAnswer(index(orig), msg[1:])
	default:
	}
	return out
}
