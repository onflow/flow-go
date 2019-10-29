package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/include -I./relic/include/low
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "dkg_include.h"
import "C"
import (
	log "github.com/sirupsen/logrus"
)

func (s *feldmanVSSQualState) receiveShare(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	log.Debugf("%d Receiving a share from %d\n", s.currentIndex, origin)
	log.Debugf("the share is %d\n", data)
	if s.xReceived {
		return invalid, nil, nil
	}
	if (len(data)) != PrKeyLengthBLS_BLS12381 {
		return invalid, nil, nil
	}
	// read the node private share
	C.bn_read_bin((*C.bn_st)(&s.x),
		(*C.uchar)(&data[0]),
		PrKeyLengthBLS_BLS12381,
	)

	s.xReceived = true
	if s.AReceived {
		result := s.verifyShare()
		if result == valid {
			return result, nil, nil
		}
		// build a complaint
		complaint := DKGToSend{
			broadcast: true,
			data:      []byte{byte(FeldmanVSSComplaint), byte(s.leaderIndex)},
		}
		return result, []DKGToSend{complaint}, nil
	}
	return valid, nil, nil
}

func (s *feldmanVSSQualState) receiveVerifVector(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	log.Debugf("%d Receiving vector from %d\n", s.currentIndex, origin)
	log.Debugf("the vector is %d\n", data)
	if s.AReceived {
		return invalid, nil, nil
	}
	if (PubKeyLengthBLS_BLS12381)*(s.threshold+1) != len(data) {
		return invalid, nil, nil
	}
	// read the verification vector
	s.A = make([]pointG2, s.threshold+1)
	readVerifVector(s.A, data)

	s.y = make([]pointG2, s.size)
	s.computePublicKeys()

	// check the (already) registered complaints
	for _, c := range s.complaints {
		if c.answerReceived {
			c.validComplaint = s.checkComplaint(c)
		}
	}

	s.AReceived = true
	if s.xReceived {
		result := s.verifyShare()
		if result == valid {
			return result, nil, nil
		}
		// build a complaint
		complaint := DKGToSend{
			broadcast: true,
			data:      []byte{byte(FeldmanVSSComplaint), byte(s.leaderIndex)},
		}
		return result, []DKGToSend{complaint}, nil
	}
	return valid, nil, nil
}

// assuming a complaint answer was received, this function returns false if the answer is valid,
// and returns true if the complaint is valid
func (s *feldmanVSSQualState) checkComplaint(c *complaint) bool {
	// TODO: check the answer
	return false
}

func (s *feldmanVSSQualState) receiveComplaint(origin index, data []byte) (DKGresult, []DKGToSend, error) {
	// TODO: check the length and return invlalid
	// first byte encodes the complainee
	complainee := data[0]
	if complainee == byte(s.leaderIndex) {
		// TODO: prepare an answer  mainly complainer:origin, answer:x[origin]
		return valid, nil, nil
	}
	// TODO: look up and/or register the complaint
	// TODO: if the answer is there, call checkcomplaint
	return valid, nil, nil
}

func (s *feldmanVSSQualState) receiveComplaintAnswer(origin index, data []byte) (DKGresult, error) {
	// TODO: check the length and return invalid and add increment the complaints number
	// TODO: if origin is current, ignore (maybe earlier)
	// TODO: if the complaint was already received, ignore it
	// TODO: if the complaint was not received yet, register the complaint
	// TODO: if the complaint answer was received, call checkcomplaint and update the number
	return invalid, nil
}
