package crypto

import (
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var messageToSign = []byte{1, 2, 3}

// This is a testing function
// It simulates processing incoming messages by a node
func tsDkgProcessChan(n int, current int, dkg []DKGstate, ts []*ThresholdSinger, chans []chan *toProcess,
	pkChan chan PublicKey, sync chan int, t *testing.T) {
	for {
		select {
		case newMsg := <-chans[current]:
			if newMsg.msgType == dkgType {
				log.Infof("%d Receiving DKG from %d:", current, newMsg.orig)
				out := dkg[current].ReceiveDKGMsg(newMsg.orig, newMsg.msg.(DKGmsg))
				out.processDkgOutput(current, dkg, chans, t)
			} else if newMsg.msgType == tsType {
				log.Infof("%d Receiving TS from %d:", current, newMsg.orig)
				verif, _, err := ts[current].ReceiveThresholdSignatureMsg(newMsg.orig, newMsg.msg.(Signature))
				assert.Nil(t, err)
				assert.True(t, verif, "the signature share is not correct")
				//verif, err = ts[current].VerifyThresholdSignature(thresholdSignature)
				//assert.Nil(t, err)
				//assert.True(t, verif, "the threshold signature is not correct")
				sync <- 0
			}

		// if timeout, finalize DKG and sign the share
		case <-time.After(time.Second):
			log.Infof("%d quit dkg \n", current)
			sk, groupPK, nodesPK, err := dkg[current].EndDKG()
			assert.Nil(t, err)
			if err != nil {
				log.Errorf("End dkg failed: %s\n", err.Error())
			}
			//pkChan <- groupPK
			ts[current], _ = NewThresholdSigner(n, SHA3_384)
			ts[current].SetKeys(sk, groupPK, nodesPK)
			ts[current].SetMessageToSign(messageToSign)
			sighShare, _ := ts[current].SignShare()
			time.Sleep(time.Second)
			broadcast(current, tsType, sighShare, chans)
		}
	}
}

// Testing Threshold Signature
func TestThresholdSignature(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Info("DKG starts")
	// number of nodes to test
	n := 5
	lead := 0
	dkg := make([]DKGstate, n)
	ts := make([]*ThresholdSinger, n)
	pkChan := make(chan PublicKey)
	sync := make(chan int)
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
		chans[i] = make(chan *toProcess, 10)
		go tsDkgProcessChan(n, i, dkg, ts, chans, pkChan, sync, t)
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
	//var pkTemp PublicKey
	//groupPK := <-pkChan
	for i := 1; i < n*(n-1); i++ {
		/*pkTemp = <-pkChan
		if !equalPublicKey(groupPK, pkTemp) {
			log.Error("Group Public key mismatch!")
		}*/
		<-sync
	}
}

func equalPublicKey(a, b PublicKey) bool {
	return true
}
