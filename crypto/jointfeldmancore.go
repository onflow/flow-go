// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"

func (s *JointFeldmanState) sumUpQualifiedKeys(qualified int) (*scalar, *pointG2, []pointG2) {
	qualifiedx, qualifiedPubKey, qualifiedy := s.getQualifiedKeys(qualified)

	// sum up x
	var jointx scalar
	C.sumScalarVector((*C.bn_st)(&jointx), (*C.bn_st)(&qualifiedx[0]), (C.int)(qualified))
	// sum up Y
	var jointPublicKey pointG2
	C.sumPointG2Vector((*C.ep2_st)(&jointPublicKey), (*C.ep2_st)(&qualifiedPubKey[0]), (C.int)(qualified))
	// sum up []y
	jointy := make([]pointG2, s.size)
	for i := 0; i < s.size; i++ {
		C.sumPointG2Vector((*C.ep2_st)(&jointy[i]), (*C.ep2_st)(&qualifiedy[i][0]), (C.int)(qualified))
	}
	return &jointx, &jointPublicKey, jointy
}

func (s *JointFeldmanState) getQualifiedKeys(qualified int) ([]scalar, []pointG2, [][]pointG2) {
	qualifiedx := make([]scalar, 0, qualified)
	qualifiedPubKey := make([]pointG2, 0, qualified)
	qualifiedy := make([][]pointG2, 0, qualified)

	for i := 0; i < s.size; i++ {
		if !s.fvss[i].disqualified {
			qualifiedx = append(qualifiedx, s.fvss[i].x)
			qualifiedPubKey = append(qualifiedPubKey, s.fvss[i].A[0])
			qualifiedy = append(qualifiedy, s.fvss[i].y)
		}
	}
	return qualifiedx, qualifiedPubKey, qualifiedy
}
