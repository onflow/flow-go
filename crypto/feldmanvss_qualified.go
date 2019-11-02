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
}

// these data are required to be stored to justify a slashing
type complaint struct {
	received       bool
	answerReceived bool
	answer         scalar
	validComplaint bool
}

func (s *feldmanVSSQualState) init() {
	s.feldmanVSSstate.init()
	s.complaints = make(map[index]*complaint)
}

func (s *feldmanVSSQualState) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	s.running = false
	// TODO: iterate on the map and check the number of valid complaints
	// if the number is more than t+1, disqualify the leader and return nil,nil,nil, nil
	errorString := ""
	// private key of the current node
	var x PrivateKey
	if s.xReceived {
		x = &PrKeyBLS_BLS12381{
			alg:    s.blsContext, // signer algo
			scalar: s.x,          // the private share
		}
	} else {
		errorString += "The private key is missing\n"
	}

	// Group public key
	var Y PublicKey
	if s.AReceived {
		Y = &PubKeyBLS_BLS12381{
			point: s.A[0],
		}
	} else {
		errorString += "The group public key is missing\n"
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	if s.y != nil {
		for i, p := range s.y {
			y[i] = &PubKeyBLS_BLS12381{
				point: p,
			}
		}
	} else {
		errorString += "The nodes public keys are missing\n"
	}
	if errorString != "" {
		return x, Y, y, cryptoError{errorString}
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
