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
	for threshold := 0; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("FeldmanVSS (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, feldmanVSS, n, threshold, happyPath)
		})
	}
}

// Testing Feldman VSS with the qualification system by simulating a network of n nodes
func testFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	// happy path, test multiple values of thresold
	for threshold := 0; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("FeldmanVSSQual (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, feldmanVSSQual, n, threshold, happyPath)
		})
	}

	n = 5
	// unhappy path, with focus on the optimal threshold value
	threshold := optimalThreshold(n)
	t.Run(fmt.Sprintf("FeldmanVSSQual UnhappyPath (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
		dkgCommonTest(t, feldmanVSSQual, n, threshold, invalidShares)
	})
}

type testCase int
type behavior int

const (
	happyPath testCase = iota
	invalidShares
	invalidVector
	invalidComplaint
	invalidComplaintAnswer

	honest behavior = iota
	manyInvalidShares
	fewInvalidShare
)

// Testing JointFeldman by simulating a network of n nodes
func testJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)

	n := 4
	// happy path, test multiple values of thresold
	for threshold := 0; threshold < n; threshold++ {
		t.Run(fmt.Sprintf("JointFeldman (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
			dkgCommonTest(t, jointFeldman, n, threshold, happyPath)
		})
	}

	n = 5
	// unhappy path, with invalid shares
	threshold := optimalThreshold(n)
	t.Run(fmt.Sprintf("JointFeldman_InvalidShares (n,t)=(%d,%d)", n, threshold), func(t *testing.T) {
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
	log.Info("DKG protocol set up")

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create n processors for all nodes
	processors := make([]testDKGProcessor, 0, n)
	for current := 0; current < n; current++ {
		list := make(int, newSigner)
		processors = append(processors, testDKGProcessor{
			current:   current,
			threshold: threshold,
			chans:     chans,
			msgType:   dkgType,
			malicious: honest,
			disqualified: list,
		})
	}

	// Update processors depending on the test
	rand := time.Now().UnixNano()
	mrand.Seed(rand)
	t.Logf("math rand seed is %d", rand)
	var r1, r2 int

	switch test {
	case invalidShares:
		r1 := mrand.Intn(n+1)      // leaders with invalid shares and will get disqualified
		r2 := mrand.Intn(n - r1 + 1) // leaders with invalid shares but will recover
		var i int
		for i := 0; i < r1; i++ {
			processors[i] = manyInvalidShares
		}
		for ; i < r1+r2; i++ {
			processors[i] = fewInvalidShares
		}
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
	read, err := rand.Read(seed)
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
	case invalidShares:
		// check the disqualified list for all non-disqualified participants
		if n-r1 > 0 {
			for i:= r1+1, i<n; i++ {
				assert.Equal(t, processors[r1].disqualified, processors[i].disqualified)
			}
		}
		// check if DKG is successful
		if r1>threshold || (n-r1) <= threshold {
			// DKG has failed, check for final errors
			for i:= r1, i<n; i++ {
				assert.Nil(t, processors[i].finalError)
			}
		}
		else {
			// DKG has succeeded, check the final keys
			for i:= r1, i<n; i++ {
				assert.True(t, processors[r1].pk.Equals(processors[i].pk),
					"2 group public keys are mismatching")
			}
		}
	}
}

// implements DKGProcessor interface
type testDKGProcessor struct {
	current int
	dkg     DKGState
	chans   []chan *message
	msgType int
	// threshold
	threshold int
	// group public key, output of DKG 
	pk PublicKey
	// final disqualified list
	disqualified []int
	// final output error of the DKG
	finalError error
	// type of malicious behavior
	malicious behavior

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
// This function simulates the behavior of a malicious node
func (proc *testDKGProcessor) InvalidShareSend(dest int, data []byte) {
	log.Infof("%d malicously Sending to %d:\n", proc.current, dest)
	// TODO: malicious send 
	// check for few or many invalid shares 
	// check destination < or > threshold
	// choose a random way for invalid share

	log.Debug(data)
	// simulate a wrong private share (the protocol should recover)
	if proc.dkg.Size() > 2 && proc.current == 0 && dest < 2 {
		data[8]++
	}
	// simulate not sending a share at all (the protocol should recover)
	if proc.dkg.Size() > 2 && proc.current == 0 && dest == 2 {
		return
	}
	newMsg := &message{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a honest message from one node to another
func (proc *testDKGProcessor) honestSend(dest int, data []byte) {
	log.Infof("%d Sending to %d:\n", proc.current, dest)
	log.Debug(data)
	newMsg := &message{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *testDKGProcessor) PrivateSend(dest int, data []byte) {
	switch proc.behavior {
	case honest:
		proc.honestSend(dest, data)
		return
	case fewInvlalidShares: 
		proc.InvalidShareSend(dest, data)
		return	
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
	log.Infof("%d wants to Disqualify %d: %s", proc.current, node, logInfo)
	proc.disqualified[node] = true
}
func (proc *testDKGProcessor) FlagMisbehavior(node int, logInfo string) {
	log.Infof("%d flags a misbehavior from %d: %s", proc.current, node, logInfo)
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
				proc.finalError := err
				proc.pk := pk
			}
			sync.Done()
			return
		}
	}
}
