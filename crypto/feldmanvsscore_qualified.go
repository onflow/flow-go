// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99
// #include "dkg_include.h"
import "C"

func (s *feldmanVSSQualState) setSharesTimeout() {
	s.sharesTimeout = true
	// if verif vector is not received, disqualify the leader
	if !s.AReceived {
		s.disqualified = true
		s.processor.Blacklist(int(s.leaderIndex))
		return
	}
	// if share is not received, make a complaint
	if !s.xReceived {
		s.complaints[s.currentIndex] = &complaint{
			received:       true,
			answerReceived: false,
		}
		data := []byte{byte(feldmanVSSComplaint), byte(s.leaderIndex)}
		s.processor.Broadcast(data)
	}
}

func (s *feldmanVSSQualState) setComplaintsTimeout() {
	s.complaintsTimeout = true
	// if more than t complaints are received, the leader is disqualified
	// regardless of the answers.
	// (at this point, all answered complaints should have been already received)
	// (i.e there is no complaint with (!c.received && c.answerReceived)
	if len(s.complaints) > s.threshold {
		s.disqualified = true
		s.processor.Blacklist(int(s.leaderIndex))
	}
}

func (s *feldmanVSSQualState) receiveShare(origin index, data []byte) {
	// check the share timeout
	if s.sharesTimeout {
		s.processor.FlagMisbehavior(int(origin), wrongProtocol)
		return
	}
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
		result := s.verifyShare()
		if result {
			return
		}
		// otherwise, build a complaint to broadcast and add it to the local
		// complaint map
		s.complaints[s.currentIndex] = &complaint{
			received:       true,
			answerReceived: false,
		}
		data := []byte{byte(feldmanVSSComplaint), byte(s.leaderIndex)}
		s.processor.Broadcast(data)
	}
}

func (s *feldmanVSSQualState) receiveVerifVector(origin index, data []byte) {
	// check the share timeout
	if s.sharesTimeout {
		s.processor.FlagMisbehavior(int(origin), wrongProtocol)
		return
	}

	// only accept the verification vector from the leader.
	if origin != s.leaderIndex {
		return
	}

	if s.AReceived {
		s.processor.FlagMisbehavior(int(origin), duplicated)
		return
	}
	if len(data) != verifVectorSize*(s.threshold+1) {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}
	// read the verification vector
	s.A = make([]pointG2, s.threshold+1)
	readVerifVector(s.A, data)

	s.y = make([]pointG2, s.size)
	s.computePublicKeys()

	s.AReceived = true
	// check the (already) registered complaints
	for complainee, c := range s.complaints {
		if c.received && c.answerReceived {
			if s.checkComplaint(complainee, c) {
				s.disqualified = true
				s.processor.Blacklist(int(s.leaderIndex))
				return
			}
		}
	}
	// check the private share
	if s.xReceived {
		result := s.verifyShare()
		if result {
			return
		}
		// otherwise, build a complaint to broadcast and add it to the local
		// complaints map
		s.complaints[s.currentIndex] = &complaint{
			received:       true,
			answerReceived: false,
		}
		data := []byte{byte(feldmanVSSComplaint), byte(s.leaderIndex)}
		s.processor.Broadcast(data)
	}
}

// assuming a complaint and its answer were received, this function returns
// - false if the answer is valid
// - true if the complaint is valid
func (s *feldmanVSSQualState) checkComplaint(complainee index, c *complaint) bool {
	// check y[complainee] == share.G2
	return C.verifyshare((*C.bn_st)(&c.answer),
		(*C.ep2_st)(&s.y[complainee])) == 0
}

// data = |complainee|
func (s *feldmanVSSQualState) receiveComplaint(origin index, data []byte) {
	// check the complaints timeout
	if s.complaintsTimeout {
		s.processor.FlagMisbehavior(int(origin), wrongProtocol)
		return
	}

	if origin == s.leaderIndex {
		return
	}

	if len(data) == 0 {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	// first byte encodes the complainee
	complainee := index(data[0])

	// if the complainee is not the leader, ignore the complaint
	if complainee != s.leaderIndex {
		return
	}

	if len(data) != complaintSize {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	c, ok := s.complaints[origin]
	// if the complaint is new, add it
	if !ok {
		s.complaints[origin] = &complaint{
			received:       true,
			answerReceived: false,
		}
		// if the complainee is the current node, prepare an answer
		if s.currentIndex == s.leaderIndex {
			data := make([]byte, complainAnswerSize+1)
			data[0] = byte(feldmanVSSComplaintAnswer)
			data[1] = byte(origin)
			ZrPolynomialImage(data[2:], s.a, origin+1, nil)
			s.complaints[origin].answerReceived = true
			s.processor.Broadcast(data)
		}
		return
	}
	// complaint is not new in the map
	// check if the complain has been already received
	if c.received {
		s.processor.FlagMisbehavior(int(origin), duplicated)
		return
	}
	c.received = true
	// first flag check is a sanity check
	if c.answerReceived && s.currentIndex != s.leaderIndex {
		s.disqualified = s.checkComplaint(origin, c)
		if s.disqualified {
			s.processor.Blacklist(int(s.leaderIndex))
		}
		return
	}
}

// answer = |complainer| private share |
func (s *feldmanVSSQualState) receiveComplaintAnswer(origin index, data []byte) {
	// check for invalid answers
	if origin != s.leaderIndex {
		return
	}

	if len(data) == 0 {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	// first byte encodes the complainee
	complainer := index(data[0])
	if int(complainer) >= s.size {
		s.processor.FlagMisbehavior(int(origin), wrongFormat)
		return
	}

	c, ok := s.complaints[complainer]
	// if the complaint is new, add it
	if !ok {
		s.complaints[complainer] = &complaint{
			received:       false,
			answerReceived: true,
		}
		// check the answer format
		if len(data) != complainAnswerSize {
			s.disqualified = true
			s.processor.Blacklist(int(s.leaderIndex))
			return
		}
		// read the complainer private share
		C.bn_read_bin((*C.bn_st)(&s.complaints[complainer].answer),
			(*C.uchar)(&data[1]),
			PrKeyLenBLS_BLS12381,
		)
		return
	}
	// complaint is not new in the map
	// check if the answer has been already received
	if c.answerReceived {
		s.processor.FlagMisbehavior(int(origin), duplicated)
		return
	}

	c.answerReceived = true
	if len(data) != complainAnswerSize {
		s.disqualified = true
		return
	}

	// first flag check is a sanity check
	if c.received {
		// read the complainer private share
		C.bn_read_bin((*C.bn_st)(&c.answer),
			(*C.uchar)(&data[1]),
			PrKeyLenBLS_BLS12381,
		)
		s.disqualified = s.checkComplaint(complainer, c)
		if s.disqualified {
			s.processor.Blacklist(int(s.leaderIndex))
		}

		// fix the share of the current node if the complaint in invalid
		if !s.disqualified && complainer == s.currentIndex {
			s.x = c.answer
		}
	}
}
