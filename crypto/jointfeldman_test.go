// +build relic

package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Testing JointFeldman by simulating a network of n nodes
func TestJointFeldman(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	log.Debug("Feldman VSS with complaints starts")
	// number of nodes to test
	n := 5
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
		processors[current].dkg, err = NewDKG(JointFeldman, n, current, &processors[current], 0)
		assert.Nil(t, err)
	}

	// create the node channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 5*n)
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
			log.Error("PK", groupPKBytes)
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
