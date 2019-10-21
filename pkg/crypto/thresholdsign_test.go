package crypto

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var messageToSign = []byte{1, 2, 3}

// This is a testing function
// It simulates processing incoming messages by a node during DKG
func tsDkgProcessChan(n int, current int, dkg []DKGstate, ts []*ThresholdSinger, chans []chan *toProcess,
	pkChan chan PublicKey, dkgSync chan int, t *testing.T) {
	for {
		select {
		case newMsg := <-chans[current]:
			log.Infof("%d Receiving DKG from %d:", current, newMsg.orig)
			out := dkg[current].ReceiveDKGMsg(newMsg.orig, newMsg.msg.(DKGmsg))
			out.processDkgOutput(current, dkg, chans, t)

		// if timeout, finalize DKG and sign the share
		case <-time.After(time.Second):
			log.Infof("%d quit dkg \n", current)
			sk, groupPK, nodesPK, err := dkg[current].EndDKG()
			assert.NotNil(t, sk)
			assert.NotNil(t, groupPK)
			assert.NotNil(t, nodesPK)
			assert.Nil(t, err)
			if err != nil {
				log.Errorf("End dkg failed: %s\n", err.Error())
			}
			pkChan <- groupPK
			ts[current], err = NewThresholdSigner(n, SHA3_384)
			assert.Nil(t, err)
			ts[current].SetKeys(sk, groupPK, nodesPK)
			ts[current].SetMessageToSign(messageToSign)
			dkgSync <- 0
			return
		}
	}
}

// This is a testing function
// It simulates processing incoming messages by a node during TS
func tsProcessChan(current int, ts []*ThresholdSinger, chans []chan *toProcess,
	tsSync chan int, t *testing.T) {
	sighShare, _ := ts[current].SignShare()
	broadcast(current, tsType, sighShare, chans)
	for {
		select {
		case newMsg := <-chans[current]:
			log.Infof("%d Receiving TS from %d:", current, newMsg.orig)
			verif, _, err := ts[current].ReceiveThresholdSignatureMsg(newMsg.orig, newMsg.msg.(Signature))
			assert.Nil(t, err)
			assert.True(t, verif,
				fmt.Sprintf("the signature share sent from %d to %d is not correct", newMsg.orig, current))
			//verif, err = ts[current].VerifyThresholdSignature(thresholdSignature)
			//assert.Nil(t, err)
			//assert.True(t, verif, "the threshold signature is not correct")

		// if timeout, finalize DKG and sign the share
		case <-time.After(time.Second):
			tsSync <- 0
			return
		}
	}
}

// Testing Threshold Signature
func TestThresholdSignature(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Info("DKG starts")
	// number of nodes to test
	n := 5
	lead := 0
	dkg := make([]DKGstate, n)
	ts := make([]*ThresholdSinger, n)
	pkChan := make(chan PublicKey)
	dkgSync := make(chan int)
	tsSync := make(chan int)
	chans := make([]chan *toProcess, n)

	// create DKG in all nodes
	for current := 0; current < n; current++ {
		var err error
		dkg[current], err = NewDKG(FeldmanVSS, n, current, lead)
		assert.Nil(t, err)
		if err != nil {
			log.Error(err.Error())
			return
		}
	}
	// create the node (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 2*n)
		go tsDkgProcessChan(n, i, dkg, ts, chans, pkChan, dkgSync, t)
	}
	// start DKG in all nodes but the leader
	seed := []byte{1, 2, 3}
	for current := 0; current < n; current++ {
		if current != lead {
			out := dkg[current].StartDKG(seed)
			out.processDkgOutput(current, dkg, chans, t)
		}
	}
	// start the leader (this avoids a data racing issue)
	out := dkg[lead].StartDKG(seed)
	out.processDkgOutput(lead, dkg, chans, t)

	// this loop synchronizes the main thread to end DKG
	var pkTemp, groupPK []byte
	for i := 0; i < n; i++ {
		if i == 0 {
			groupPK, _ = (<-pkChan).Encode()
		} else {
			pkTemp, _ = (<-pkChan).Encode()
			assert.Equal(t, groupPK, pkTemp)
		}
		<-dkgSync
	}

	log.Info("TS starts")
	for i := 0; i < n; i++ {
		go tsProcessChan(i, ts, chans, tsSync, t)
	}
	// this loop synchronizes the main thread to end TS
	for i := 1; i < n; i++ {
		<-tsSync
	}
}
