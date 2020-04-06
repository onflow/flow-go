// +build relic

package crypto

import (
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDKG(t *testing.T) {
	t.Run("FeldmanVSS", testFeldmanVSSSimple)
	t.Run("FeldmanVSS with Qualified set", testFeldmanVSSQual)
	t.Run("FeldmanVSS Unhappy Path", testFeldmanVSSQualUnhappyPath)
	t.Run("Joint Feldman", testJointFeldman)
	t.Run("Joint Feldman Unhappy Path", testJointFeldmanUnhappyPath)
}

// Testing the happy path of Feldman VSS by simulating a network of n nodes
func testFeldmanVSSSimple(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5)
	}

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
	}
	dkgCommonTest(t, FeldmanVSS, processors)
}

// Testing Feldman VSS with the qualification system by simulating a network of n nodes
func testFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
	}
	dkgCommonTest(t, FeldmanVSSQual, processors)
}

// Testing Feldman VSS with the qualification system by simulating a network of n nodes
func testFeldmanVSSQualUnhappyPath(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current:   current,
			chans:     chans,
			msgType:   dkgType,
			malicious: true,
		})
	}
	dkgCommonTest(t, FeldmanVSSQual, processors)
}

// Testing JointFeldman by simulating a network of n nodes
func testJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
	}
	dkgCommonTest(t, JointFeldman, processors)
}

func testJointFeldmanUnhappyPath(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *message, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 5*n)
	}

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current:   current,
			chans:     chans,
			msgType:   dkgType,
			malicious: true,
		})
	}
	dkgCommonTest(t, JointFeldman, processors)
}

func dkgCommonTest(t *testing.T, dkg DKGType, processors []testDKGProcessor) {
	log.Info("DKG protocol starts")
	// number of nodes to test
	n := len(processors)
	lead := 0
	var sync sync.WaitGroup

	// create DKG in all nodes
	for current := 0; current < n; current++ {
		var err error
		processors[current].dkg, err = NewDKG(dkg, n, current,
			&processors[current], lead)
		require.Nil(t, err)
	}

	phase := 0
	if dkg == FeldmanVSS {
		phase = 2
	}

	// start DKG in all nodes
	// start listening on the channels
	h, _ := NewHasher(SHA3_256)
	seed := h.ComputeHash([]byte{1, 2, 3})
	sync.Add(n)
	for current := 0; current < n; current++ {
		// start dkg could also run in parallel
		// but they are run sequentially to avoid having non-deterministic
		// output (the PRG used is common)
		err := processors[current].dkg.StartDKG(seed)
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
	log.Info("PK", processors[0].pkBytes)
	for i := 1; i < n; i++ {
		assert.Equal(t, processors[i].pkBytes, processors[0].pkBytes,
			"2 group public keys are mismatching")
	}
}

// implements DKGProcessor interface
type testDKGProcessor struct {
	current   int
	dkg       DKGstate
	chans     []chan *message
	msgType   int
	pkBytes   []byte
	malicious bool
	// only used when testing the threshold signature stateful api
	ts *ThresholdSigner
	// only used when testing the threshold signature statless api
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
func (proc *testDKGProcessor) maliciousSend(dest int, data []byte) {
	log.Infof("%d malicously Sending to %d:\n", proc.current, dest)
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
func (proc *testDKGProcessor) Send(dest int, data []byte) {
	if proc.malicious {
		proc.maliciousSend(dest, data)
		return
	}
	proc.honestSend(dest, data)
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

func (proc *testDKGProcessor) Blacklist(node int) {
	log.Infof("%d wants to blacklist %d", proc.current, node)
}
func (proc *testDKGProcessor) FlagMisbehavior(node int, logData string) {
	log.Infof("%d flags a misbehavior from %d: %s", proc.current, node, logData)
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
				_, pk, _, err := proc.dkg.EndDKG()
				assert.Nil(t, err, "end dkg error should be nil")
				if pk == nil {
					proc.pkBytes = []byte{}
				} else {
					proc.pkBytes, _ = pk.Encode()
				}
			}
			sync.Done()
			return
		}
	}
}
