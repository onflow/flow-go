package crypto

// Implemets Feldman Verifiable Secret Sharing using BLS G2 group.

type feldmanVSSstate struct {
	// common DKG state
	*DKGcommon
	// node leader index
	leaderIndex int
	// internal context of BLS on BLS12381A
	blsContext *BLS_BLS12381Algo
	// Polynomial P = a_0 + a_1*x + .. + a_t*x^t  in Zr[X], the vector size is (t+1)
	// a_0 is the group private key
	a []scalar
	// Public data of the group, the vector size is (t+1)
	// A_0 is the group public key
	A []pointG2
	// Private share of the current node
	x         scalar
	xReceived bool
	// Public keys of the group nodes, the vector size is (n)
	y []pointG2
}

func (s *feldmanVSSstate) init() {
	s.isrunning = false

	blsSigner, _ := NewSignatureAlgo(BLS_BLS12381)
	s.blsContext = blsSigner.(*BLS_BLS12381Algo)

	s.A = nil
	s.y = nil
	s.xReceived = false
}

func (s *feldmanVSSstate) StartDKG() (*DKGoutput, error) {
	s.isrunning = true
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		seed := []byte{1, 2, 3}
		return s.generateShares(seed), nil
	}
	return new(DKGoutput), nil
}

func (s *feldmanVSSstate) EndDKG() (PrKey, PubKey, []PubKey, error) {
	s.isrunning = false
	// private key of the current node
	var x PrKey
	if s.xReceived {
		x = &PrKeyBLS_BLS12381{
			alg:    s.blsContext, // signer algo
			scalar: s.x,          // the private share
		}
	} else {
		x = nil
	}
	// Group public key
	var Y PubKey
	if s.A != nil {
		Y = &PubKeyBLS_BLS12381{
			point: s.A[0],
		}
	} else {
		Y = nil
	}
	// The nodes public keys
	y := make([]PubKey, s.size)
	if s.y != nil {
		for i, p := range s.y {
			y[i] = &PubKeyBLS_BLS12381{
				point: p,
			}
		}
	} else {
		for i := 0; i < s.size; i++ {
			y[i] = nil
		}
	}
	return x, Y, y, nil
}

func (s *feldmanVSSstate) ProcessDKGmsg(orig int, msg DKGmsg) *DKGoutput {
	out := &(DKGoutput{
		action: []DKGToSend{},
		err:    nil,
	})

	if !s.isrunning || len(msg) == 0 {
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
