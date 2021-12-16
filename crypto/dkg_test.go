// +build relic

package crypto

import (
	"fmt"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gt *testing.T

func TestDKG(t *testing.T) {
	t.Run("FeldmanVSSSimple", testFeldmanVSSSimple)
	t.Run("FeldmanVSSQual", testFeldmanVSSQual)
	t.Run("JointFeldman", testJointFeldman)
}

// optimal threshold (t) to allow the largest number of malicious participants (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}

// Testing the happy path of Feldman VSS by simulating a network of n participants
func testFeldmanVSSSimple(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("FeldmanVSS (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, feldmanVSS, n, threshold, happyPath)
		})
	}
}

type testCase int

const (
	happyPath testCase = iota
	invalidShares
	invalidVector
	invalidComplaint
	invalidComplaintAnswer
	duplicatedMessages
)

type behavior int

const (
	honest behavior = iota
	manyInvalidShares
	fewInvalidShares
	invalidVectorBroadcast
	invalidComplaintBroadcast
	timeoutedComplaintBroadcast
	invalidSharesComplainTrigger
	invalidComplaintAnswerBroadcast
	duplicatedSendAndBroadcast
)

// Testing Feldman VSS with the qualification system by simulating a network of n participants
func testFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	// happy path, test multiple values of thresold
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("FeldmanVSSQual_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, feldmanVSSQual, n, threshold, happyPath)
		})
	}

	// unhappy path, with focus on the optimal threshold value
	n = 5
	threshold := optimalThreshold(n)
	// unhappy path, with invalid shares
	t.Run(fmt.Sprintf("FeldmanVSSQual_InvalidShares_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, feldmanVSSQual, n, threshold, invalidShares)
	})
	// unhappy path, with invalid vector
	t.Run(fmt.Sprintf("FeldmanVSSQual_InvalidVector_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, feldmanVSSQual, n, threshold, invalidVector)
	})
	// unhappy paths with invalid complaints and complaint answers
	// are only tested within joint feldman.
}

// Testing JointFeldman by simulating a network of n participants
func testJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	var threshold int
	// happy path, test multiple values of thresold
	for threshold = MinimumThreshold; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("JointFeldman_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, jointFeldman, n, threshold, happyPath)
		})
	}

	// unhappy path, with focus on the optimal threshold value
	n = 5
	threshold = optimalThreshold(n)
	// unhappy path, with invalid shares
	t.Run(fmt.Sprintf("JointFeldman_InvalidShares_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, invalidShares)
	})
	// unhappy path, with invalid vector
	t.Run(fmt.Sprintf("JointFeldman_InvalidVector_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, invalidVector)
	})
	// unhappy path, with invalid complaints
	t.Run(fmt.Sprintf("JointFeldman_InvalidComplaints_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, invalidComplaint)
	})
	// unhappy path, with invalid complaint answers
	t.Run(fmt.Sprintf("JointFeldman_InvalidComplaintAnswers_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, invalidComplaintAnswer)
	})
	// unhappy path, with duplicated messages (all types)
	t.Run(fmt.Sprintf("JointFeldman_DuplicatedMessages_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, duplicatedMessages)
	})
}

// Supported Key Generation protocols
const (
	feldmanVSS = iota
	feldmanVSSQual
	jointFeldman
)

func newDKG(dkg int, size int, threshold int, myIndex int,
	processor DKGProcessor, leaderIndex int) (DKGState, error) {
	switch dkg {
	case feldmanVSS:
		return NewFeldmanVSS(size, threshold, myIndex, processor, leaderIndex)
	case feldmanVSSQual:
		return NewFeldmanVSSQual(size, threshold, myIndex, processor, leaderIndex)
	case jointFeldman:
		return NewJointFeldman(size, threshold, myIndex, processor)
	default:
		return nil, fmt.Errorf("non supported protocol")
	}
}

func dkgCommonTest(t *testing.T, dkg int, n int, threshold int, test testCase) {
	gt = t
	log.Info("DKG protocol set up")

	// create the participant channels
	chans := make([]chan *message, n)
	lateChansTimeout1 := make([]chan *message, n)
	lateChansTimeout2 := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
		lateChansTimeout1[i] = make(chan *message, 5*n)
		lateChansTimeout2[i] = make(chan *message, 5*n)
	}

	// number of leaders in the protocol
	var leaders int
	if dkg == jointFeldman {
		leaders = n
	} else {
		leaders = 1
	}

	// create n processors for all participants
	processors := make([]testDKGProcessor, 0, n)
	for current := 0; current < n; current++ {
		list := make([]bool, leaders)
		processors = append(processors, testDKGProcessor{
			current:           current,
			chans:             chans,
			lateChansTimeout1: lateChansTimeout1,
			lateChansTimeout2: lateChansTimeout2,
			protocol:          dkgType,
			malicious:         honest,
			disqualified:      list,
		})
	}

	// Update processors depending on the test
	rand := time.Now().UnixNano()
	mrand.Seed(rand)
	t.Logf("math rand seed is %d", rand)
	// r1 and r2 is the number of malicious participants, each group with a slight diffrent behavior.
	// - r1 participants of indices 0 to r1-1 behave maliciously and will get disqualified by honest participants.
	// - r2 participants of indices r1 to r1+r2-1 will behave maliciously at first but will recover and won't be
	// disqualified by honest participants. The r2 participants may or may not obtain correct protocol results.
	var r1, r2 int
	// h is the index of the first honest participant. All participant with indices greater than or equal to h are honest.
	// Checking the final protocol results is done for honest participants only.
	// Whether the r2 participants belong to the honest participants or not depend on the malicious behavior (detailed below).
	var h int

	switch test {
	case happyPath:
		// r1 = r2 = 0

	case invalidShares:
		r1 = mrand.Intn(leaders + 1)      // leaders with invalid shares and will get disqualified
		r2 = mrand.Intn(leaders - r1 + 1) // leaders with invalid shares but will recover
		h = r1

		var i int
		for i = 0; i < r1; i++ {
			processors[i].malicious = manyInvalidShares
		}
		for ; i < r1+r2; i++ {
			processors[i].malicious = fewInvalidShares
		}
		t.Logf("%d participants will be disqualified, %d other participants will recover\n", r1, r2)

	case invalidVector:
		r1 = 1 + mrand.Intn(leaders) // leaders with invalid vector and will get disqualified
		h = r1

		// in this case r2 = 0
		for i := 0; i < r1; i++ {
			processors[i].malicious = invalidVectorBroadcast
		}
		t.Logf("%d participants will be disqualified\n", r1)

	case invalidComplaint:
		r1 = 1 + mrand.Intn(leaders-1) // participants with invalid complaints and will get disqualified.
		// r1>= 1 to have at least one malicious leader, and r1<leadrers-1 to leave space for the trigger leader below.
		r2 = mrand.Intn(leaders - r1) // participants with timeouted complaints: they are considered qualified by honest participants
		// but their results are invalid
		h = r1 + r2 // r2 shouldn't be verified for protocol correctness

		for i := 0; i < r1; i++ {
			processors[i].malicious = invalidComplaintBroadcast
		}
		for i := r1; i < r1+r2; i++ {
			processors[i].malicious = timeoutedComplaintBroadcast
		}
		// The participant (r1+r2) will send wrong shares and cause the 0..r1+r2-1 leaders to send complaints.
		// This participant doesn't risk getting disqualified as the complaints against them
		// are invalid and won't count. The participant doesn't even answer the complaint.
		processors[r1+r2].malicious = invalidSharesComplainTrigger
		t.Logf("%d participants will be disqualified, %d other participants won't be disqualified.\n", r1, r2)

	case invalidComplaintAnswer:
		r1 = 1 + mrand.Intn(leaders-1) // participants with invalid complaint answers and will get disqualified.
		// r1>= 1 to have at least one malicious leader, and r1<leadrers-1 to leave space for the complaint sender.
		h = r1
		// the 0..r1-1 leaders will send invalid shares to n-1 to trigger complaints.
		for i := 0; i < r1; i++ {
			processors[i].malicious = invalidComplaintAnswerBroadcast
		}
		t.Logf("%d participants will be disqualified\n", r1)
	case duplicatedMessages:
		// r1 = r2 = 0
		// participant 0 will send duplicated shares, verif vector and complaint to all participants
		processors[0].malicious = duplicatedSendAndBroadcast
		// participant 1 is a complaint trigger, it sents a wrong share to 0 to trigger a complaint.
		// it also sends duplicated complaint answers.
		processors[1].malicious = invalidSharesComplainTrigger

	default:
		panic("test case not supported")
	}

	// number of participants to test
	lead := 0
	var sync sync.WaitGroup

	// create DKG in all participants
	for current := 0; current < n; current++ {
		var err error
		processors[current].dkg, err = newDKG(dkg, n, threshold,
			current, &processors[current], lead)
		require.NoError(t, err)
	}

	phase := 0
	if dkg == feldmanVSS {
		phase = 2
	}

	// start DKG in all participants
	// start listening on the channels
	seed := make([]byte, SeedMinLenDKG)
	read, err := mrand.Read(seed)
	require.Equal(t, read, SeedMinLenDKG)
	require.NoError(t, err)
	sync.Add(n)

	log.Info("DKG protocol starts")

	for current := 0; current < n; current++ {
		processors[current].startSync.Add(1)
		go dkgRunChan(&processors[current], &sync, t, phase)
	}

	for current := 0; current < n; current++ {
		// start dkg in parallel
		// ( one common PRG is used for all instances which causes a race
		//	in generating randoms and leads to non-deterministic keys. If deterministic keys
		//  are required, switch to sequential calls to dkg.Start()  )
		go func(current int) {
			err := processors[current].dkg.Start(seed)
			require.Nil(t, err)
			processors[current].startSync.Done() // avoids reading messages when a dkg instance hasn't started yet
		}(current)
	}
	phase++

	// sync the two timeouts and start the next phase
	for ; phase <= 2; phase++ {
		sync.Wait()
		// post processing required for timeout edge case tests
		go timeoutPostProcess(processors, t, phase)
		sync.Add(n)
		for current := 0; current < n; current++ {
			go dkgRunChan(&processors[current], &sync, t, phase)
		}
	}

	// synchronize the main thread to end all DKGs
	sync.Wait()

	// assertions and results:

	// check the disqualified list for all non-disqualified participants
	expected := make([]bool, leaders)
	for i := 0; i < r1; i++ {
		expected[i] = true
	}

	for i := h; i < n; i++ {
		t.Logf("participant %d is not disqualified, its disqualified list is:\n", i)
		t.Log(processors[i].disqualified)
		assert.Equal(t, expected, processors[i].disqualified)
	}
	// check if DKG is successful
	if (dkg == jointFeldman && (r1 > threshold || (n-r1) <= threshold)) ||
		(dkg == feldmanVSSQual && r1 == 1) { // case of a single leader
		t.Logf("dkg failed, there are %d disqualified participants\n", r1)
		// DKG failed, check for final errors
		for i := r1; i < n; i++ {
			err := processors[i].finalError
			assert.Error(t, err)
			assert.True(t, IsDKGFailureError(err))
		}
	} else {
		t.Logf("dkg succeeded, there are %d disqualified participants\n", r1)
		// DKG has succeeded, check for final errors
		for i := h; i < n; i++ {
			assert.NoError(t, processors[i].finalError)
		}
		// DKG has succeeded, check the final keys
		for i := h; i < n; i++ {
			assert.True(t, processors[h].pk.Equals(processors[i].pk),
				"2 group public keys are mismatching")
		}
	}

}

// time after which a silent channel causes switching to the next dkg phase
const phaseSwitchTimeout = 200 * time.Millisecond

// This is a testing function
// It simulates processing incoming messages by a participant
// it assumes proc.dkg is already running
func dkgRunChan(proc *testDKGProcessor,
	sync *sync.WaitGroup, t *testing.T, phase int) {
	for {
		select {
		// if a message is received, handle it
		case newMsg := <-proc.chans[proc.current]:
			proc.startSync.Wait() // avoids reading a message when the receiving dkg instance
			// hasn't started yet.
			if newMsg.channel == private {
				err := proc.dkg.HandlePrivateMsg(newMsg.orig, newMsg.data)
				require.Nil(t, err)
			} else {
				err := proc.dkg.HandleBroadcastMsg(newMsg.orig, newMsg.data)
				require.Nil(t, err)
			}
		// if no message is received by the channel, call the DKG timeout
		case <-time.After(phaseSwitchTimeout):
			switch phase {
			case 0:
				log.Infof("%d shares phase ended\n", proc.current)
				err := proc.dkg.NextTimeout()
				require.Nil(t, err)
			case 1:
				log.Infof("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				require.Nil(t, err)
			case 2:
				log.Infof("%d dkg ended \n", proc.current)
				_, pk, _, err := proc.dkg.End()
				proc.finalError = err
				proc.pk = pk
			}
			sync.Done()
			return
		}
	}
}

// post processing required for some edge case tests
func timeoutPostProcess(processors []testDKGProcessor, t *testing.T, phase int) {
	switch phase {
	case 1:
		for i := 0; i < len(processors); i++ {
			go func(i int) {
				for len(processors[0].lateChansTimeout1[i]) != 0 {
					// to test timeouted messages, late messages are copied to the main channels
					msg := <-processors[0].lateChansTimeout1[i]
					processors[0].chans[i] <- msg
				}
			}(i)
		}
	case 2:
		for i := 0; i < len(processors); i++ {
			go func(i int) {
				for len(processors[0].lateChansTimeout2[i]) != 0 {
					// to test timeouted messages, late messages are copied to the main channels
					msg := <-processors[0].lateChansTimeout2[i]
					processors[0].chans[i] <- msg
				}
			}(i)
		}
	}
}

// implements DKGProcessor interface
type testDKGProcessor struct {
	// instnce of DKG
	dkg DKGState
	// index of the current participant in the protocol
	current int
	// group public key, output of DKG
	pk PublicKey
	// final disqualified list
	disqualified []bool
	// final output error of the DKG
	finalError error
	// type of malicious behavior
	malicious behavior
	// start DKG syncer
	startSync sync.WaitGroup

	// main message channels
	chans []chan *message
	// extra channels for late messges with regards to the first timeout, and second timeout
	lateChansTimeout1 []chan *message
	lateChansTimeout2 []chan *message
	// type of the protocol
	protocol int

	// only used when testing the threshold signature stateful api
	ts   *blsThresholdSignatureParticipant
	keys *statelessKeys
}

const (
	dkgType int = iota
	tsType
)

const (
	broadcast int = iota
	private
)

type message struct {
	orig     int
	protocol int
	channel  int
	data     []byte
}

func (proc *testDKGProcessor) Disqualify(participant int, logInfo string) {
	gt.Logf("%d disqualifies %d, %s\n", proc.current, participant, logInfo)
	proc.disqualified[participant] = true
}

func (proc *testDKGProcessor) FlagMisbehavior(participant int, logInfo string) {
	gt.Logf("%d flags a misbehavior from %d: %s", proc.current, participant, logInfo)
}

// This is a testing function
// it simulates sending a message from one participant to another
func (proc *testDKGProcessor) PrivateSend(dest int, data []byte) {
	go func() {
		log.Infof("%d sending to %d", proc.current, dest)
		if proc.malicious == fewInvalidShares || proc.malicious == manyInvalidShares ||
			proc.malicious == invalidSharesComplainTrigger || proc.malicious == invalidComplaintAnswerBroadcast ||
			proc.malicious == duplicatedSendAndBroadcast {
			proc.invalidShareSend(dest, data)
			return
		}
		proc.honestSend(dest, data)
	}()
}

// This is a testing function
// it simulates sending a honest message from one participant to another
func (proc *testDKGProcessor) honestSend(dest int, data []byte) {
	gt.Logf("%d honestly sending to %d:\n%x\n", proc.current, dest, data)
	newMsg := &message{proc.current, proc.protocol, private, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a malicious message from one participant to another
// This function simulates the behavior of a malicious participant.
func (proc *testDKGProcessor) invalidShareSend(dest int, data []byte) {

	// check the behavior
	var recipients int // number of recipients to send invalid shares to
	switch proc.malicious {
	case manyInvalidShares:
		recipients = proc.dkg.Threshold() + 1 //  t < recipients <= n
	case fewInvalidShares:
		recipients = proc.dkg.Threshold() //  0 <= recipients <= t
	case invalidSharesComplainTrigger:
		recipients = proc.current // equal to r1+r2, which causes all r1+r2 to complain
	case invalidComplaintAnswerBroadcast:
		recipients = 0 // treat this case separately as the complaint trigger is the participant n-1
	case duplicatedSendAndBroadcast:
		proc.honestSend(dest, data)
		proc.honestSend(dest, data)
		return
	default:
		panic("invalid share send not supported")
	}

	newMsg := &message{proc.current, proc.protocol, private, data}
	// check destination
	if (dest < recipients) || (proc.current < recipients && dest < recipients+1) ||
		(proc.malicious == invalidComplaintAnswerBroadcast && dest == proc.dkg.Size()-1) {
		// choose a random reason for an invalid share
		coin := mrand.Intn(6)
		gt.Logf("%d maliciously sending to %d, coin is %d\n", proc.current, dest, coin)
		switch coin {
		case 0:
			// value doesn't match the verification vector
			newMsg.data[8]++
		case 1:
			// invalid length
			newMsg.data = newMsg.data[:1]
		case 2:
			// invalid value
			for i := 0; i < len(newMsg.data); i++ {
				newMsg.data[i] = 0xFF
			}
		case 3:
			// do not send the share at all
			return
		case 4:
			// wrong header: equivalent to not sending the share at all
			newMsg.data[0] = byte(feldmanVSSVerifVec)
		case 5:
			// message will be sent after the shares timeout and will be considered late
			// by the receiver. All late messages go into a separate channel and will be sent to
			// the main channel after the shares timeout.
			proc.lateChansTimeout1[dest] <- newMsg
			return
		}
	} else {
		gt.Logf("turns out to be a honest send\n%x\n", data)
		proc.chans[dest] <- newMsg
	}
}

// This is a testing function
// it simulates broadcasting a message from one participant to all participants
func (proc *testDKGProcessor) Broadcast(data []byte) {
	go func() {
		log.Infof("%d Broadcasting:", proc.current)

		if data[0] == byte(feldmanVSSVerifVec) && proc.malicious == invalidVectorBroadcast {
			proc.invalidVectorBroadcast(data)
		} else if data[0] == byte(feldmanVSSComplaint) &&
			(proc.malicious == invalidComplaintBroadcast || proc.malicious == timeoutedComplaintBroadcast) {
			proc.invalidComplaintBroadcast(data)
		} else if data[0] == byte(feldmanVSSComplaintAnswer) && proc.malicious == invalidComplaintAnswerBroadcast {
			proc.invalidComplaintAnswerBroadcast(data)
		} else if proc.malicious == duplicatedSendAndBroadcast ||
			(data[0] == byte(feldmanVSSComplaintAnswer) && proc.malicious == invalidSharesComplainTrigger) {
			// the complaint trigger also sends duplicated complaint answers
			proc.honestBroadcast(data)
			proc.honestBroadcast(data)
		} else {
			proc.honestBroadcast(data)
		}
	}()
}

func (proc *testDKGProcessor) honestBroadcast(data []byte) {
	gt.Logf("%d honestly broadcasting:\n%x\n", proc.current, data)
	newMsg := &message{proc.current, proc.protocol, broadcast, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func (proc *testDKGProcessor) invalidVectorBroadcast(data []byte) {
	newMsg := &message{proc.current, proc.protocol, broadcast, data}

	// choose a random reason of an invalid vector
	coin := mrand.Intn(5)
	gt.Logf("%d malicious vector broadcast, coin is %d\n", proc.current, coin)
	switch coin {
	case 0:
		// invalid point serialization
		newMsg.data[1] = 0xFF
	case 1:
		// invalid length
		newMsg.data = newMsg.data[:5]
	case 2:
		// do not send the vector at all
		return
	case 3:
		// wrong header, equivalent to not sending at all
		newMsg.data[0] = byte(feldmanVSSShare)
	case 4:
		// send the vector after the first timeout, equivalent to not sending at all
		// as the vector should be ignored.
		for i := 0; i < proc.dkg.Size(); i++ {
			if i != proc.current {
				proc.lateChansTimeout1[i] <- newMsg
			}
		}
		return
	}
	gt.Logf("%x\n", newMsg.data)
	for i := 0; i < proc.dkg.Size(); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func (proc *testDKGProcessor) invalidComplaintBroadcast(data []byte) {
	newMsg := &message{proc.current, proc.protocol, broadcast, data}

	if proc.malicious == invalidComplaintBroadcast {

		// choose a random reason for an invalid complaint
		coin := mrand.Intn(2)
		gt.Logf("%d malicious complaint broadcast, coin is %d\n", proc.current, coin)
		switch coin {
		case 0:
			// invalid complainee
			newMsg.data[1] = byte(proc.dkg.Size() + 1)
		case 1:
			// invalid length
			newMsg.data = make([]byte, complaintSize+5)
			copy(newMsg.data, data)
		}
		gt.Logf("%x\n", newMsg.data)
		for i := 0; i < len(proc.chans); i++ {
			if i != proc.current {
				proc.chans[i] <- newMsg
			}
		}
	} else if proc.malicious == timeoutedComplaintBroadcast {
		gt.Logf("%d timeouted complaint broadcast\n", proc.current)
		// send the complaint after the second timeout, equivalent to not sending at all
		// as the complaint should be ignored.
		for i := 0; i < len(proc.chans); i++ {
			if i != proc.current {
				proc.lateChansTimeout2[i] <- newMsg
			}
		}
		return
	}
}

func (proc *testDKGProcessor) invalidComplaintAnswerBroadcast(data []byte) {
	newMsg := &message{proc.current, proc.protocol, broadcast, data}

	// choose a random reason for an invalid complaint
	coin := mrand.Intn(3)
	gt.Logf("%d malicious complaint answer broadcast, coin is %d\n", proc.current, coin)
	switch coin {
	case 0:
		// invalid complainee
		newMsg.data[1] = byte(proc.dkg.Size() + 1)
	case 1:
		// invalid length
		newMsg.data = make([]byte, complaintAnswerSize+5)
		copy(newMsg.data, data)
	case 2:
		// no answer at all
		return
	}
	//gt.Logf("%x\n", newMsg.data)
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func TestErrorTypes(t *testing.T) {
	t.Run("dkgFailureError", func(t *testing.T) {
		failureError := dkgFailureErrorf("some error")
		otherError := fmt.Errorf("some error")
		assert.True(t, IsDKGFailureError(failureError))
		assert.False(t, IsDKGFailureError(otherError))
	})
}
