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

// optimal threshold (t) to allow the largest number of malicious nodes (m)
// assuming the protocol requires:
//   m<=t for unforgeability
//   n-m>=t+1 for robustness
func optimalThreshold(size int) int {
	return (size - 1) / 2
}

// Testing the happy path of Feldman VSS by simulating a network of n nodes
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
type behavior int

const (
	happyPath testCase = iota
	invalidShares
	invalidVector
	invalidComplaint
	invalidComplaintAnswer
)

const (
	honest behavior = iota
	manyInvalidShares
	fewInvalidShares
)

// Testing Feldman VSS with the qualification system by simulating a network of n nodes
func testFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	// happy path, test multiple values of thresold
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("FeldmanVSSQual_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, feldmanVSSQual, n, threshold, happyPath)
		})
	}

	n = 5
	// unhappy path, with focus on the optimal threshold value
	threshold := optimalThreshold(n)
	t.Run(fmt.Sprintf("FeldmanVSSQual_InvalidShares_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, feldmanVSSQual, n, threshold, invalidShares)
	})
}

// Testing JointFeldman by simulating a network of n nodes
func testJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	// happy path, test multiple values of thresold
	for threshold := MinimumThreshold; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("JointFeldman_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, jointFeldman, n, threshold, happyPath)
		})
	}

	n = 5
	// unhappy path, with invalid shares
	threshold := optimalThreshold(n)
	t.Run(fmt.Sprintf("JointFeldman_InvalidShares_(n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, jointFeldman, n, threshold, invalidShares)
	})
}

// Supported Key Generation protocols
const (
	feldmanVSS = iota
	feldmanVSSQual
	jointFeldman
)

func newDKG(dkg int, size int, threshold int, currentIndex int,
	processor DKGProcessor, leaderIndex int) (DKGState, error) {
	switch dkg {
	case feldmanVSS:
		return NewFeldmanVSS(size, threshold, currentIndex, processor, leaderIndex)
	case feldmanVSSQual:
		return NewFeldmanVSSQual(size, threshold, currentIndex, processor, leaderIndex)
	case jointFeldman:
		return NewJointFeldman(size, threshold, currentIndex, processor)
	default:
		return nil, fmt.Errorf("non supported protocol")
	}
}

func dkgCommonTest(t *testing.T, dkg int, n int, threshold int, test testCase) {
	gt = t
	log.Info("DKG protocol set up")

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// number of leaders in the protocol
	var leaders int
	if dkg == jointFeldman {
		leaders = n
	} else {
		leaders = 1
	}

	// create n processors for all nodes
	processors := make([]testDKGProcessor, 0, n)
	for current := 0; current < n; current++ {
		list := make([]bool, leaders)
		processors = append(processors, testDKGProcessor{
			current:      current,
			chans:        chans,
			msgType:      dkgType,
			malicious:    honest,
			disqualified: list,
		})
	}

	// Update processors depending on the test
	rand := time.Now().UnixNano()
	mrand.Seed(rand)
	t.Logf("math rand seed is %d", rand)
	var r1, r2 int

	switch test {
	case happyPath:
		// r1 = r2 = 0
		break
	case invalidShares:
		r1 = mrand.Intn(leaders + 1)      // leaders with invalid shares and will get disqualified
		r2 = mrand.Intn(leaders - r1 + 1) // leaders with invalid shares but will recover
		var i int
		for i = 0; i < r1; i++ {
			processors[i].malicious = manyInvalidShares
		}
		for ; i < r1+r2; i++ {
			processors[i].malicious = fewInvalidShares
		}
		t.Logf("%d participants will be disqualified, %d participants will recover\n", r1, r2)
	default:
		panic("test case not supported")
	}

	// number of nodes to test
	lead := 0
	var sync sync.WaitGroup

	// create DKG in all nodes
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

	// start DKG in all nodes
	// start listening on the channels
	seed := make([]byte, SeedMinLenDKG)
	read, err := mrand.Read(seed)
	require.Equal(t, read, SeedMinLenDKG)
	require.NoError(t, err)
	sync.Add(n)

	log.Info("DKG protocol starts")

	for current := 0; current < n; current++ {
		// start dkg could also run in parallel
		// but they are run sequentially to avoid having non-deterministic
		// output (the PRG used is common)
		err := processors[current].dkg.Start(seed)
		require.Nil(t, err)
		go dkgRunChan(&processors[current], &sync, t, phase)
	}
	phase++

	// sync the two timeouts and start the next phase
	for ; phase <= 2; phase++ {
		sync.Wait()
		sync.Add(n)
		for current := 0; current < n; current++ {
			go dkgRunChan(&processors[current], &sync, t, phase)
		}
	}

	// synchronize the main thread to end all DKGs
	sync.Wait()

	switch test {
	case happyPath:
		fallthrough
	case invalidShares:
		// check the disqualified list for all non-disqualified participants
		expected := make([]bool, leaders)
		for i := 0; i < r1; i++ {
			expected[i] = true
		}
		for i := r1; i < n; i++ {
			t.Logf("node %d is not disqualified, its disqualified list is:\n", i)
			t.Log(processors[i].disqualified)
			assert.Equal(t, expected, processors[i].disqualified)
		}
		// check if DKG is successful
		if (dkg == jointFeldman && (r1 > threshold || (n-r1) <= threshold)) ||
			(r1 == 1) { // case of a single leader
			t.Logf("dkg failed, there are %d disqualified nodes\n", r1)
			// DKG has failed, check for final errors
			for i := r1; i < n; i++ {
				assert.Error(t, processors[i].finalError)
			}
		} else {
			t.Logf("dkg succeeded, there are %d disqualified nodes\n", r1)
			// DKG has succeeded, check for final errors
			for i := r1; i < n; i++ {
				assert.NoError(t, processors[i].finalError)
			}
			// DKG has succeeded, check the final keys
			for i := r1; i < n; i++ {
				assert.True(t, processors[r1].pk.Equals(processors[i].pk),
					"2 group public keys are mismatching")
			}
		}
	default:
		log.Error("test case not supported")
	}
}

// implements DKGProcessor interface
type testDKGProcessor struct {
	// instnce of DKG
	dkg DKGState
	// index of the current node in the protocol
	current int
	// group public key, output of DKG
	pk PublicKey
	// final disqualified list
	disqualified []bool
	// final output error of the DKG
	finalError error
	// type of malicious behavior
	malicious behavior

	chans   []chan *message
	msgType int

	// only used when testing the threshold signature stateful api
	ts   *thresholdSigner
	keys *statelessKeys
}

const (
	dkgType int = iota
	tsType
)

type message struct {
	orig    int
	msgType int
	data    []byte
}

// This is a testing function
// it simulates sending a malicious message from one node to another
// This function simulates the behavior of a malicious node.
func (proc *testDKGProcessor) InvalidShareSend(dest int, data []byte) {

	// check the behavior
	var recipients int // number of recipients to send invalid shares
	if proc.malicious == manyInvalidShares {
		recipients = proc.dkg.Threshold() + 1 //  t < recipients <= n
	} else { // fewInvalidShares
		recipients = proc.dkg.Threshold() //  0 <= recipients <= t
	}

	newMsg := &message{proc.current, proc.msgType, data}
	// check destination
	if dest < recipients || (proc.current < recipients && dest < recipients+1) {
		// choose a random method to invalid share
		coin := mrand.Intn(100)
		gt.Logf("malicious send, coin is %d\n", coin%4)
		switch coin % 4 {
		case 0:
			// value does't match the verification vector
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
		}
	} else {
		gt.Logf("turns out to be a honest send\n")
	}
	gt.Logf("%x\n", newMsg.data)
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a honest message from one node to another
func (proc *testDKGProcessor) honestSend(dest int, data []byte) {
	gt.Logf("honest send\n")
	gt.Logf("%x\n", data)
	newMsg := &message{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *testDKGProcessor) PrivateSend(dest int, data []byte) {
	gt.Logf("%d sending to %d", proc.current, dest)
	switch proc.malicious {
	case honest:
		proc.honestSend(dest, data)
		return
	case fewInvalidShares:
		fallthrough
	case manyInvalidShares:
		proc.InvalidShareSend(dest, data)
		return
	default:
		log.Error()
	}
}

// This is a testing function
// it simulates broadcasting a message from one node to all nodes
func (proc *testDKGProcessor) Broadcast(data []byte) {
	log.Infof("%d Broadcasting:", proc.current)
	log.Debug(data)
	newMsg := &message{proc.current, proc.msgType, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func (proc *testDKGProcessor) Disqualify(node int, logInfo string) {
	gt.Logf("%d disqualifies %d, %s\n", proc.current, node, logInfo)
	proc.disqualified[node] = true
}

func (proc *testDKGProcessor) FlagMisbehavior(node int, logInfo string) {
	gt.Logf("%d flags a misbehavior from %d: %s", proc.current, node, logInfo)
}

// This is a testing function
// It simulates processing incoming messages by a node
// it assumes proc.dkg is already running
func dkgRunChan(proc *testDKGProcessor,
	sync *sync.WaitGroup, t *testing.T, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving from %d:", proc.current, newMsg.orig)
			err := proc.dkg.HandleMsg(newMsg.orig, newMsg.data)
			require.Nil(t, err)
		// if timeout, stop and finalize
		case <-time.After(200 * time.Millisecond):
			switch phase {
			case 0:
				log.Infof("%d shares phase ended \n", proc.current)
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
