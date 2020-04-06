// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "dkg_include.h"
import "C"

import (
	"errors"
)

func (s *feldmanVSSstate) generateShares(seed []byte) error {
	err := seedRelic(seed)
	if err != nil {
		return err
	}
	// Generate a polyomial P in Zr[X] of degree t
	s.a = make([]scalar, s.threshold+1)
	s.A = make([]pointG2, s.threshold+1)
	s.y = make([]pointG2, s.size)
	for i := 0; i < s.threshold+1; i++ {
		randZr(&s.a[i])
		_G2scalarGenMult(&s.A[i], &s.a[i])
	}

	// compute the shares
	for i := index(1); int(i) <= s.size; i++ {
		// the-leader-own share
		if i-1 == s.currentIndex {
			xdata := make([]byte, shareSize)
			ZrPolynomialImage(xdata, s.a, i, &s.y[i-1])
			C.bn_read_bin((*C.bn_st)(&s.x),
				(*C.uchar)(&xdata[0]),
				PrKeyLenBLS_BLS12381,
			)
			continue
		}
		// the-other-node shares
		data := make([]byte, shareSize+1)
		data[0] = byte(feldmanVSSShare)
		ZrPolynomialImage(data[1:], s.a, i, &s.y[i-1])
		s.processor.Send(int(i-1), data)
	}
	// broadcast the vector
	vectorSize := verifVectorSize * (s.threshold + 1)
	data := make([]byte, vectorSize+1)
	data[0] = byte(feldmanVSSVerifVec)
	writeVerifVector(data[1:], s.A)
	s.processor.Broadcast(data)

	s.AReceived = true
	s.xReceived = true
	s.validKey = true
	return nil
}

func (s *feldmanVSSstate) receiveShare(origin index, data []byte) {
	// only accept private shares from the leader.
	if origin != s.leaderIndex {
		return
	}

	if s.xReceived {
		s.processor.FlagMisbehavior(int(origin), duplicated)
		return
	}

	if (len(data)) != shareSize {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	// read the node private share
	C.bn_read_bin((*C.bn_st)(&s.x),
		(*C.uchar)(&data[0]),
		PrKeyLenBLS_BLS12381,
	)

	s.xReceived = true
	if s.AReceived {
		s.validKey = s.verifyShare()
	}
}

func (s *feldmanVSSstate) receiveVerifVector(origin index, data []byte) {
	// only accept the verification vector from the leader.
	if origin != s.leaderIndex {
		return
	}
	if s.AReceived {
		s.processor.FlagMisbehavior(int(origin), duplicated)
		return
	}
	if verifVectorSize*(s.threshold+1) != len(data) {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}
	// read the verification vector
	s.A = make([]pointG2, s.threshold+1)
	err := readVerifVector(s.A, data)
	if err != nil {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	s.y = make([]pointG2, s.size)
	s.computePublicKeys()

	s.AReceived = true
	if s.xReceived {
		s.validKey = s.verifyShare()
	}
}

// ZrPolynomialImage computes P(x) = a_0 + a_1*x + .. + a_n*x^n (mod r) in Z/Zr
// r being the order of G1
// P(x) is written in dest, while g2^P(x) is written in y
// x being a small integer
func ZrPolynomialImage(dest []byte, a []scalar, x index, y *pointG2) {
	C.Zr_polynomialImage((*C.uchar)(&dest[0]),
		(*C.ep2_st)(y),
		(*C.bn_st)(&a[0]), (C.int)(len(a)),
		(C.uint8_t)(x),
	)
}

// writeVerifVector exports a vector A into an array of bytes
// assuming the array length matches the vector length
func writeVerifVector(dest []byte, A []pointG2) {
	C.ep2_vector_write_bin((*C.uchar)(&dest[0]),
		(*C.ep2_st)(&A[0]),
		(C.int)(len(A)),
	)
}

// readVerifVector imports A vector from an array of bytes,
// assuming the slice length matches the vector length
func readVerifVector(A []pointG2, src []byte) error {
	if C.ep2_vector_read_bin((*C.ep2_st)(&A[0]),
		(*C.uchar)(&src[0]),
		(C.int)(len(A)),
	) != valid {
		return errors.New("the verifcation vector does not encode public keys correctly")
	}
	return nil
}

func (s *feldmanVSSstate) verifyShare() bool {
	// check y[current] == x.G2
	return C.verifyshare((*C.bn_st)(&s.x),
		(*C.ep2_st)(&s.y[s.currentIndex])) == 1
}

// computePublicKeys extracts the nodes public keys from the verification vector
// y[i] = Q(i+1) for all nodes i, with:
//  Q(x) = A_0 + A_1*x + ... +  A_n*x^n  in G2
func (s *feldmanVSSstate) computePublicKeys() {
	C.G2_polynomialImages(
		(*C.ep2_st)(&s.y[0]), (C.int)(len(s.y)),
		(*C.ep2_st)(&s.A[0]), (C.int)(len(s.A)),
	)
}
