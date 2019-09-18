package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"
import (
	"fmt"
	"unsafe"
)

// TODO: remove -wall after reaching a stable version
// TDOO: enable QUIET in relic

func (s *feldmanVSSstate) generateShares(seed []byte) *DKGoutput {
	seedRelic(seed)
	// Generate a polyomial P in Zr[X]
	s.a = make([]scalar, s.threshold+1)
	for i := 0; i < s.threshold+1; i++ {
		//randZr(&(s.a[i]))
		s.a[i].setInt(1)
	}

	// prepare the DKGToSend slice and compute the shares
	out := &(DKGoutput{
		result: nonApplicable,
		err:    nil,
	})

	out.action = make([]DKGToSend, s.size-1)
	for i := 0; i < len(out.action); i++ {
		toSend := &(out.action[i])
		toSend.broadcast = false
		toSend.dest = index(s.currentIndex, i)
		shareSize := s.blsContext.PrKeySize()
		data := make([]byte, shareSize+1)
		data[0] = byte(FeldmanVSSshare)
		index := index(s.currentIndex, i)
		copy(data[1:], polynomialImage(s.a, index, shareSize))
		toSend.data = DKGmsg(data)
	}
	return out
}

func (s *feldmanVSSstate) receiveShare(origin int, data []byte) (dkgResult, error) {
	fmt.Printf("%d Receiving a share from %d, the share is %d\n", s.currentIndex, origin, data[0])
	return valid, nil
}

// computes P(x) = a_0 + a_1*x + .. + a_n x^n (mod p)
// p being the order of G1
// x being a small integer
func polynomialImage(a []scalar, x int, outputSize int) []byte {
	s := make([]byte, outputSize)

	C._polynomialImage((*C.uchar)((unsafe.Pointer)(&s[0])),
		(*C.bn_st)(&a[0]), (C.int)(len(a)),
		(C.int)(x),
	)
	return s
}
