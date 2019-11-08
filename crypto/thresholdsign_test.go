// +build relic

package crypto

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var messageToSign = []byte{1, 2, 3}

// This is a testing function
// It simulates processing incoming messages by a node during DKG
func tsDkgRunChan(n int, proc *testDKGProcessor,
	pkChan chan PublicKey, dkgSync chan int, t *testing.T) {
	// wait till DKG starts
	for proc.dkg.Running() == false {
	}
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Debugf("%d Receiving DKG from %d:", proc.current, newMsg.orig)
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.msg)
			assert.Nil(t, err)

		// if timeout, finalize DKG and sign the share
		case <-time.After(time.Second):
			log.Debugf("%d quit dkg \n", proc.current)
			sk, groupPK, nodesPK, err := proc.dkg.EndDKG()
			assert.NotNil(t, sk)
			assert.NotNil(t, groupPK)
			assert.NotNil(t, nodesPK)
			assert.Nil(t, err, "End dkg failed: %v\n", err)
			pkChan <- groupPK
			proc.ts, err = NewThresholdSigner(n, SHA3_384)
			assert.Nil(t, err)
			proc.ts.SetKeys(sk, groupPK, nodesPK)
			proc.ts.SetMessageToSign(messageToSign)
			dkgSync <- 0
			return
		}
	}
}

// This is a testing function
// It simulates processing incoming messages by a node during TS
func tsRunChan(proc *testDKGProcessor, tsSync chan int, t *testing.T) {
	// Sign a share and broadcast it
	sighShare, _ := proc.ts.SignShare()
	proc.msgType = tsType
	proc.Broadcast(sighShare)
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			log.Infof("%d Receiving TS from %d:", proc.current, newMsg.orig)
			verif, thresholdSignature, err := proc.ts.ReceiveThresholdSignatureMsg(
				newMsg.orig, newMsg.msg)
			assert.Nil(t, err)
			assert.True(t, verif,
				"the signature share sent from %d to %d is not correct", newMsg.orig,
				proc.current)
			if thresholdSignature != nil {
				verif, err = proc.ts.VerifyThresholdSignature(thresholdSignature)
				assert.Nil(t, err)
				assert.True(t, verif, "the threshold signature is not correct")
				if verif {
					log.Debugf("%d reconstructed a valid signature: %d\n", proc.current,
						thresholdSignature)
				}
			}

		// if timeout, finalize TS
		case <-time.After(time.Second):
			tsSync <- 0
			return
		}
	}
}

// Testing Threshold Signature
// keys are generated using simple Feldman VSS
func TestThresholdSignature(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Info("DKG starts")
	// number of nodes to test
	n := 5
	lead := 0
	pkChan := make(chan PublicKey)
	dkgSync := make(chan int)
	tsSync := make(chan int)
	chans := make([]chan *toProcess, n)
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
		if err != nil {
			log.Error(err.Error())
			return
		}
	}

	// create the node (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 2*n)
		go tsDkgRunChan(n, &processors[i], pkChan, dkgSync, t)
	}
	// start DKG in all nodes but the leader
	seed := []byte{1, 2, 3}
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
	}

	// this loop synchronizes the main thread to end DKG
	var pkTemp, groupPK []byte
	for i := 0; i < n; i++ {
		if i == 0 {
			groupPK, _ = (<-pkChan).Encode()
			log.Info("PK", groupPK)
		} else {
			pkTemp, _ = (<-pkChan).Encode()
			assert.Equal(t, groupPK, pkTemp)
		}
		<-dkgSync
	}

	// Start TS
	log.Info("TS starts")
	for i := 0; i < n; i++ {
		go tsRunChan(&processors[i], tsSync, t)
	}
	// this loop synchronizes the main thread to end TS
	for i := 1; i < n; i++ {
		<-tsSync
	}
}
