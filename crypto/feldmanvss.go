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
	// true if the private share is valid
	validKey bool
}

func (s *feldmanVSSstate) init() {
	s.running = false

	blsSigner, _ := NewSigner(BLS_BLS12381)
	s.blsContext = blsSigner.(*BLS_BLS12381Algo)

	s.y = nil
	s.xReceived = false
	s.AReceived = false
}

func (s *feldmanVSSstate) StartDKG(seed []byte) error {
	if s.running {
		return cryptoError{"dkg is already running"}
	}

	s.running = true
	// Generate shares if necessary
	if s.leaderIndex == s.currentIndex {
		return s.generateShares(seed)
	}
	return nil
}

func (s *feldmanVSSstate) EndDKG() (PrivateKey, PublicKey, []PublicKey, error) {
	if !s.running {
		return nil, nil, nil, cryptoError{"dkg is not running"}
	}
	s.running = false
	if !s.validKey {
		return nil, nil, nil, cryptoError{"keys are not correct"}
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
	shareSize = PrKeyLenBLS_BLS12381
	// the actual verifVectorSize depends on the state and should be:
	// PubKeyLenBLS_BLS12381*(t+1)
	verifVectorSize = PubKeyLenBLS_BLS12381
)

func (s *feldmanVSSstate) ReceiveDKGMsg(orig int, msg []byte) error {
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

	// msg = |tag| Data |
	switch dkgMsgTag(msg[0]) {
	case feldmanVSSShare:
		s.receiveShare(index(orig), msg[1:])
	case feldmanVSSVerifVec:
		s.receiveVerifVector(index(orig), msg[1:])
	default:
		s.processor.FlagMisbehavior(orig, wrongFormat)
	}
	return nil
}

func (s *feldmanVSSstate) Disqualify(node int) error {
	if !s.running {
		return cryptoError{"dkg is not running"}
	}
	if node >= s.Size() {
		return cryptoError{"wrong input"}
	}
	if index(node) == s.leaderIndex {
		s.validKey = false
	}
	return nil
}
