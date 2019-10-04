package crypto

// Implements Feldman Verifiable Secret Sharing using BLS G2 group.

type feldmanVSSstate struct {
	// common DKG state
	*dkgCommon
	// node leader index
	leaderIndex int
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

	s.A = nil
	s.y = nil
	s.xReceived = false
	s.AReceived = false
}

func (s *feldmanVSSstate) StartDKG(seed []byte) *DKGoutput {
	s.running = true
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		return s.generateShares(seed)
	}
	out := &DKGoutput{
		result: valid,
		err:    nil,
	}
	return out
}

func (s *feldmanVSSstate) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	s.running = false
	// private key of the current node
	var x PrivateKey
	if s.xReceived {
		x = &PrKeyBLS_BLS12381{
			alg:    s.blsContext, // signer algo
			scalar: s.x,          // the private share
		}
	}

	// Group public key
	var Y PublicKey
	if s.A != nil {
		Y = &PubKeyBLS_BLS12381{
			point: s.A[0],
		}
	}
	// The nodes public keys
	y := make([]PublicKey, s.size)
	if s.y != nil {
		for i, p := range s.y {
			y[i] = &PubKeyBLS_BLS12381{
				point: p,
			}
		}
	}
	return x, Y, y, nil
}

func (s *feldmanVSSstate) ProcessDKGmsg(orig int, msg DKGmsg) *DKGoutput {
	out := &DKGoutput{
		action: []DKGToSend{},
		err:    nil,
	}

	if !s.running || len(msg) == 0 {
		out.result = invalid
		return out
	}

	switch dkgMsgTag(msg[0]) {
	case FeldmanVSSshare:
		out.result, out.err = s.receiveShare(orig, msg[1:])
	case FeldmanVSSVerifVec:
		out.result, out.err = s.receiveVerifVector(orig, msg[1:])
	default:
		out.result = invalid
	}
	return out
}
