// +build relic

package crypto

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	dkgType int = iota
	tsType
)

type toProcess struct {
	orig    int
	msgType int
	msg     []byte
}

// implements DKGprocessor interface
type testDKGProcessor struct {
	current   int
	dkg       DKGstate
	chans     []chan *toProcess
	msgType   int
	ts        *ThresholdSigner // only used when testing a threshold signature
	malicious bool
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
	newMsg := &toProcess{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a honest message from one node to another
func (proc *testDKGProcessor) honestSend(dest int, data []byte) {
	log.Infof("%d Sending to %d:\n", proc.current, dest)
	log.Debug(data)
	newMsg := &toProcess{proc.current, proc.msgType, data}
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
	newMsg := &toProcess{proc.current, proc.msgType, data}
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
func dkgRunChan(proc *testDKGProcessor,
	pkChan chan PublicKey, sync chan int, t *testing.T, phase int) {
	// wait till DKG starts
	for proc.dkg.Running() == false {
		//fmt.Println("euh")
	}
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving from %d:", proc.current, newMsg.orig)
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.msg)
			assert.Nil(t, err)
		// if timeout, stop and finalize
		case <-time.After(time.Second):
			switch phase {
			case 0:
				log.Infof("%d shares phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 1:
				log.Infof("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 2:
				log.Infof("%d dkg ended \n", proc.current)
				_, pk, _, err := proc.dkg.EndDKG()
				assert.Nil(t, err, "end dkg error should be nil")
				pkChan <- pk
			}
			sync <- 0
			return
		}
	}
}

// Testing the happy path of Feldman VSS by simulating a network of n nodes
func TestFeldmanVSSSimple(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *toProcess, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 5)
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
func TestFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *toProcess, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 5*n)
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
func TestFeldmanVSSQualUnhappyPath(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	n := 5
	processors := make([]testDKGProcessor, 0, n)

	// create the node channels
	chans := make([]chan *toProcess, n)
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 5*n)
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

func dkgCommonTest(t *testing.T, dkg DKGType, processors []testDKGProcessor) {
	log.Info("DKG protocol starts")
	// number of nodes to test
	n := len(processors)
	lead := 0
	pkChan := make(chan PublicKey)
	sync := make(chan int)

	// create DKG in all nodes
	for current := 0; current < n; current++ {
		var err error
		processors[current].dkg, err = NewDKG(dkg, n, current,
			&processors[current], lead)
		assert.Nil(t, err)
	}

	var phase int
	if dkg == FeldmanVSS {
		phase = 2
	}

	// start listening on the channels
	for i := 0; i < n; i++ {
		go dkgRunChan(&processors[i], pkChan, sync, t, phase)
	}
	phase++

	// start DKG in all nodes
	seed := []byte{1, 2, 3}
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
	}

	// sync the two timeouts and start the next phase
	for ; phase <= 2; phase++ {
		for i := 0; i < n; i++ {
			<-sync
		}
		for i := 0; i < n; i++ {
			go dkgRunChan(&processors[i], pkChan, sync, t, phase)
		}
	}

	// this loop synchronizes the main thread to end all DKGs
	var tempPK, groupPK PublicKey
	var tempPKBytes, groupPKBytes []byte

	for i := 0; i < n; i++ {
		if i == 0 {
			groupPK = <-pkChan
			if groupPK == nil {
				groupPKBytes = []byte{}
			} else {
				groupPKBytes, _ = groupPK.Encode()
			}
			log.Info("PK", groupPKBytes)
		} else {
			tempPK = <-pkChan
			if tempPK == nil {
				tempPKBytes = []byte{}
			} else {
				tempPKBytes, _ = tempPK.Encode()
			}
			//log.Info("PK", tempPKBytes)
			assert.Equal(t, groupPKBytes, tempPKBytes, "2 group public keys are mismatching")
		}
		<-sync
	}
}
