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
	current int
	dkg     DKGstate
	chans   []chan *toProcess
	msgType int
	ts      *ThresholdSigner // only used when testing a threshold signature
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *testDKGProcessor) faultySend(orig int, dest int, msgType int, msg []byte,
	chans []chan *toProcess) {
	log.Infof("%d Sending to %d:\n", orig, dest)
	log.Debug(msg)
	if orig == 0 && (dest < 3) {
		msg[8] = 255
	}
	if orig == 0 && (dest == 3) {
		return
	}
	newMsg := &toProcess{orig, msgType, msg}
	chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *testDKGProcessor) Send(dest int, data []byte) {
	log.Infof("%d Sending to %d:\n", proc.current, dest)
	log.Debug(data)
	newMsg := &toProcess{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
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
	}
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving from %d:", proc.current, newMsg.orig)
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.msg)
			assert.Nil(t, err)
		// if timeout, stop and finalize
		case <-time.After(time.Second):
			if phase == 0 {
				log.Infof("%d shares phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			}
			if phase == 1 {
				log.Infof("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			}
			if phase == 2 {
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
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 5
	lead := 0
	pkChan := make(chan PublicKey)
	chans := make([]chan *toProcess, n)
	sync := make(chan int)
	processors := make([]testDKGProcessor, 0, n)

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
		// create DKG in all nodes
		var err error
		processors[current].dkg, err = NewDKG(FeldmanVSS, n, current,
			&processors[current], lead)
		assert.Nil(t, err)
	}

	// create the node channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 10)
		go dkgRunChan(&processors[i], pkChan, sync, t, 2)
	}
	// start DKG in all nodes
	seed := []byte{1, 2, 3}
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
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
			assert.Equal(t, groupPKBytes, tempPKBytes, "2 group public keys are mismatching")
		}
		<-sync
	}
}

// Testing Feldman VSS with the qualification system by simulating a network of n nodes
func TestFeldmanVSSQual(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS with complaints starts")
	// number of nodes to test
	n := 5
	lead := 0
	pkChan := make(chan PublicKey)
	chans := make([]chan *toProcess, n)
	sync := make(chan int)
	processors := make([]testDKGProcessor, 0, n)

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, testDKGProcessor{
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
		// create DKG in all nodes
		var err error
		processors[current].dkg, err = NewDKG(FeldmanVSSQual, n, current,
			&processors[current], lead)
		assert.Nil(t, err)
	}
	// create the node channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 10)
		go dkgRunChan(&processors[i], pkChan, sync, t, 0)
	}
	// start DKG in all nodes
	seed := []byte{1, 2, 3}
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
	}

	// sync the first timeout at all nodes and start the second phase
	for i := 0; i < n; i++ {
		<-sync
	}
	for i := 0; i < n; i++ {
		go dkgRunChan(&processors[i], pkChan, sync, t, 1)
	}

	// sync the secomd timeout at all nodes and start the last phase
	for i := 0; i < n; i++ {
		<-sync
	}
	for i := 0; i < n; i++ {
		go dkgRunChan(&processors[i], pkChan, sync, t, 2)
	}

	// this loop synchronizes the main thread to end all DKGs
	var tempPK, groupPK PublicKey
	var tempPKBytes, groupPKBytes []byte
	// TODO: check the reconstructed key is equal to a_0
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
			assert.Equal(t, groupPKBytes, tempPKBytes, "2 group public keys are mismatching")
		}
		<-sync
	}
}
