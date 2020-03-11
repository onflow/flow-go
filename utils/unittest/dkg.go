// +build relic

package unittest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/crypto"
)

const (
	dkgType int = iota
)

var messageToSign = []byte{1, 2, 3}

type message struct {
	orig    int
	msgType int
	data    []byte
}

// This stucture holds the keys and is needed for the stateless test
type statelessKeys struct {
	// the current node private key (a DKG output)
	currentPrivateKey crypto.PrivateKey
	// the group public key (a DKG output)
	groupPublicKey crypto.PublicKey
	// the group public key shares (a DKG output)
	publicKeyShares []crypto.PublicKey
}

// implements DKGProcessor interface
type TestDKGProcessor struct {
	t         *testing.T
	current   int
	dkg       crypto.DKGstate
	chans     []chan *message
	msgType   int
	pkBytes   []byte
	malicious bool
	// only used when testing the threshold signature stateful api
	ts *crypto.ThresholdSigner
	// only used when testing the threshold signature statless api
	keys *statelessKeys
}

// This is a testing function
// it simulates sending a malicious message from one node to another
// This function simulates the behavior of a malicious node
func (proc *TestDKGProcessor) maliciousSend(dest int, data []byte) {
	proc.t.Logf("%d malicously Sending to %d:\n", proc.current, dest)
	proc.t.Log(data)
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
func (proc *TestDKGProcessor) honestSend(dest int, data []byte) {
	proc.t.Logf("%d Sending to %d:\n", proc.current, dest)
	proc.t.Log(data)
	newMsg := &message{proc.current, proc.msgType, data}
	proc.chans[dest] <- newMsg
}

// This is a testing function
// it simulates sending a message from one node to another
func (proc *TestDKGProcessor) Send(dest int, data []byte) {
	if proc.malicious {
		proc.maliciousSend(dest, data)
		return
	}
	proc.honestSend(dest, data)
}

// This is a testing function
// it simulates broadcasting a message from one node to all nodes
func (proc *TestDKGProcessor) Broadcast(data []byte) {
	proc.t.Logf("%d Broadcasting:", proc.current)
	proc.t.Log(data)
	newMsg := &message{proc.current, proc.msgType, data}
	for i := 0; i < len(proc.chans); i++ {
		if i != proc.current {
			proc.chans[i] <- newMsg
		}
	}
}

func (proc *TestDKGProcessor) Blacklist(node int) {
	proc.t.Logf("%d wants to blacklist %d", proc.current, node)
}
func (proc *TestDKGProcessor) FlagMisbehavior(node int, logData string) {
	proc.t.Logf("%d flags a misbehavior from %d: %s", proc.current, node, logData)
}

func GenDKGKeys(t *testing.T, n int) ([]crypto.PrivateKey, crypto.PublicKey, []crypto.PublicKey) {
	lead := 0
	var wg sync.WaitGroup
	chans := make([]chan *message, n)
	processors := make([]TestDKGProcessor, 0, n)

	// create n processors for all nodes
	for current := 0; current < n; current++ {
		processors = append(processors, TestDKGProcessor{
			t:       t,
			current: current,
			chans:   chans,
			msgType: dkgType,
		})
		// create DKG in all nodes
		var err error
		processors[current].dkg, err = crypto.NewDKG(crypto.JointFeldman, n, current,
			&processors[current], lead)
		assert.Nil(t, err)
	}

	// create the node (buffered) communication channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *message, 2*n)
	}
	// start DKG in all nodes but the leader
	seed := []byte{1, 2, 3}
	wg.Add(n)
	for current := 0; current < n; current++ {
		err := processors[current].dkg.StartDKG(seed)
		assert.Nil(t, err)
		go tsDkgRunChan(&processors[current], &wg, t, 0)
	}

	// sync the 2 timeouts at all nodes and start the next phase
	for phase := 1; phase <= 2; phase++ {
		wg.Wait()
		wg.Add(n)
		for current := 0; current < n; current++ {
			go tsDkgRunChan(&processors[current], &wg, t, phase)
		}
	}
	// synchronize the main thread to end DKG
	wg.Wait()
	for i := 1; i < n; i++ {
		assert.Equal(t, processors[i].pkBytes, processors[0].pkBytes,
			"2 group public keys are mismatching")
	}

	privateKeys := make([]crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		privateKeys[i] = processors[i].keys.currentPrivateKey
	}
	groupKey := processors[0].keys.groupPublicKey
	publicKeyShares := processors[0].keys.publicKeyShares
	return privateKeys, groupKey, publicKeyShares
}

// This is a testing function
// It simulates processing incoming messages by a node during DKG
// It assumes proc.dkg is already running
func tsDkgRunChan(proc *TestDKGProcessor,
	sync *sync.WaitGroup, t *testing.T, phase int) {
	for {
		select {
		case newMsg := <-proc.chans[proc.current]:
			proc.t.Logf("%d Receiving DKG from %d:", proc.current, newMsg.orig)
			err := proc.dkg.ReceiveDKGMsg(newMsg.orig, newMsg.data)
			assert.Nil(t, err)

		// if timeout, finalize DKG and sign the share
		case <-time.After(200 * time.Millisecond):
			switch phase {
			case 0:
				proc.t.Logf("%d shares phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 1:
				proc.t.Logf("%d complaints phase ended \n", proc.current)
				err := proc.dkg.NextTimeout()
				assert.Nil(t, err)
			case 2:
				proc.t.Logf("%d dkg ended \n", proc.current)
				sk, groupPK, nodesPK, err := proc.dkg.EndDKG()
				assert.NotNil(t, sk)
				assert.NotNil(t, groupPK)
				assert.NotNil(t, nodesPK)
				assert.Nil(t, err, "End dkg failed: %v\n", err)
				if groupPK == nil {
					proc.pkBytes = []byte{}
				} else {
					proc.pkBytes, _ = groupPK.Encode()
				}
				n := proc.dkg.Size()
				kmac := crypto.NewBLS_KMAC(crypto.ThresholdSignatureTag)
				proc.ts, err = crypto.NewThresholdSigner(n, proc.current, kmac)
				assert.Nil(t, err)
				proc.ts.SetKeys(sk, groupPK, nodesPK)
				proc.ts.SetMessageToSign(messageToSign)
				// needed to test the statless api
				proc.keys = &statelessKeys{sk, groupPK, nodesPK}
			}
			sync.Done()
			return
		}
	}
}
