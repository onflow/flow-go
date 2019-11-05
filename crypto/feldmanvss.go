// +build relic

package crypto

// Implements Feldman Verifiable Secret Sharing using BLS set up on BLS381 curve.
// Private keys are Zr elements while public keys are G2 elements

type feldmanVSSstate struct {
	// common DKG state
	*dkgCommon
	// node leader index
	leaderIndex index
	// internal context of BLS on BLS12381A
	blsContext *BLS_BLS12381Algo
	// Polynomial P = a_0 + a_1*x + .. + a_t*x^t  in Zr[X], the vector size is (t+1)
	// a_0 is the group private key
	a []scalar
	// Public data of the group, the vector size is (t+1)
	// A_0 is the group public key
	A         []pointG2
	AReceived bool
	// Private share of the current node
	x         scalar
	xReceived bool
	// Public keys of the group nodes, the vector size is (n)
	y []pointG2
}

func (s *feldmanVSSstate) init() {
	s.running = false

	blsSigner, _ := NewSigner(BLS_BLS12381)
	s.blsContext = blsSigner.(*BLS_BLS12381Algo)

	s.y = nil
	s.xReceived = false
	s.AReceived = false
}

func (s *feldmanVSSstate) StartDKG(seed []byte) *DKGoutput {
	if !s.running {
		s.running = true
		// Generate shares if necessary
		if s.leaderIndex == s.currentIndex {
			return s.generateShares(seed)
		}
	}
	out := &DKGoutput{
		result: valid,
		err:    nil,
	}
	return out
}

func (s *feldmanVSSstate) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	s.running = false
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
	shareSize = PrKeyLenBLS_BLS12381
	// the actual verifVectorSize depends on the state and should be:
	// PubKeyLenBLS_BLS12381*(t+1)
	verifVectorSize = PubKeyLenBLS_BLS12381
)

func (s *feldmanVSSstate) ReceiveDKGMsg(orig int, msg DKGmsg) *DKGoutput {
	out := &DKGoutput{
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

	// msg = |tag| Data |
	switch dkgMsgTag(msg[0]) {
	case FeldmanVSSshare:
		out.result = s.receiveShare(index(orig), msg[1:])
	case FeldmanVSSVerifVec:
		out.result = s.receiveVerifVector(index(orig), msg[1:])
	default:
		out.result = invalid
	}
	return out
}
