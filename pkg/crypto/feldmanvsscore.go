package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"
import "fmt"

// TODO: remove -wall after reaching a stable version
// TDOO: enable QUIET in relic

func (s *feldmanVSSstate) generateShares(seed []byte) *DKGoutput {
	seedRelic(seed)
	// Generate a polyomial P in Zr[X]
	s.a = make([]scalar, s.threshold+1)
	for i := 0; i < s.threshold+1; i++ {
		randZr(&(s.a[i]))
		//s.a[i].setInt(1)
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
		share := index(s.currentIndex, i) //:= polynomialImage(a []scalar, i)
		toSend.data = DKGmsg([]byte{byte(FeldmanVSSshare), byte(share)})
	}
	return out
}

func (s *feldmanVSSstate) receiveShare(origin int, data []byte) (dkgResult, error) {
	fmt.Printf("%d Receiving a share from %d", s.currentIndex, origin)
	fmt.Printf("the share is %d\n", data[0])
	return valid, nil
}
