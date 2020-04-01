// +build relic

package crypto

// Implements Feldman Verifiable Secret Sharing using BLS setup on BLS381 curve.
// a complaint system is added to qualify/disqualify the leader
// Private keys are Zr elements while public keys are G2 elements

type feldmanVSSQualState struct {
	// feldmanVSSstate state
	*feldmanVSSstate
	// complaints received against the leader:
	// the key is the origin of the complaint
	// a complaint will be created if a complaint message or an answer was
	// broadcasted, a complaint will be checked only when both the
	// complaint message and the answer were broadcasted
	complaints map[index]*complaint
	// is the leader disqualified
	disqualified bool
	// Timeout to receive shares and verification vector
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
func (s *feldmanVSSQualState) NextTimeout() error {
	if !s.running {
		return cryptoError{"dkg protocol is not running"}
	}
	// if leader is already disqualified, there is nothing to do
	if s.disqualified {
		if s.sharesTimeout {
			s.complaintsTimeout = true
		} else {
			s.sharesTimeout = true
		}
		return nil
	}
	if !s.sharesTimeout && !s.complaintsTimeout {
		s.setSharesTimeout()
		return nil
	}
	if s.sharesTimeout && !s.complaintsTimeout {
		s.setComplaintsTimeout()
		return nil
	}
	return cryptoError{"next timeout should be to end DKG protocol"}
}

// EndDKG ends the protocol and returns the keys
// This is also a timeout to receiving all complaint answers
func (s *feldmanVSSQualState) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.running {
		return nil, nil, nil, cryptoError{"dkg protocol is not running"}
	}
	if !s.sharesTimeout || !s.complaintsTimeout {
		return nil, nil, nil,
			cryptoError{"two timeouts should be set before ending dkg"}
	}
	s.running = false
	// check if a complaint has remained without an answer
	// a leader is disqualified if a complaint was never answered
	if !s.disqualified {
		for _, c := range s.complaints {
			if c.received && !c.answerReceived {
				s.disqualified = true
				s.processor.Blacklist(int(s.leaderIndex))
				break
			}
		}
	}

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
		alg:   s.blsContext,
		point: s.A[0],
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	for i, p := range s.y {
		y[i] = &PubKeyBLS_BLS12381{
			alg:   s.blsContext,
			point: p,
		}
	}
	return x, Y, y, nil
}

const (
	complaintSize      = 1
	complainAnswerSize = 1 + PrKeyLenBLS_BLS12381
)

func (s *feldmanVSSQualState) ReceiveDKGMsg(orig int, msg []byte) error {
	if !s.running {
		return cryptoError{"dkg is not running"}
	}
	if orig >= s.Size() || orig < 0 {
		return cryptoError{"wrong input"}
	}

	if len(msg) == 0 {
		s.processor.FlagMisbehavior(orig, wrongFormat)
		return nil
	}

	// In case a broadcasted message is received by the origin node,
	// the message is just ignored
	if s.currentIndex == index(orig) {
		return nil
	}

	// if leader is already disqualified, ignore the message
	if s.disqualified {
		return nil
	}

	switch dkgMsgTag(msg[0]) {
	case feldmanVSSShare:
		s.receiveShare(index(orig), msg[1:])
	case feldmanVSSVerifVec:
		s.receiveVerifVector(index(orig), msg[1:])
	case feldmanVSSComplaint:
		s.receiveComplaint(index(orig), msg[1:])
	case feldmanVSSComplaintAnswer:
		s.receiveComplaintAnswer(index(orig), msg[1:])
	default:
		s.processor.FlagMisbehavior(orig, wrongFormat)
	}
	return nil
}

func (s *feldmanVSSQualState) Disqualify(node int) error {
	if !s.running {
		return cryptoError{"dkg is not running"}
	}
	if node >= s.Size() {
		return cryptoError{"wrong input"}
	}
	if index(node) == s.leaderIndex {
		s.disqualified = true
	}
	return nil
}
