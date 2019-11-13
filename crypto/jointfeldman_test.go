// +build relic

package crypto

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

// Testing JointFeldman by simulating a network of n nodes
func TestJointFeldman(t *testing.T) {
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
	dkgCommonTest(t, JointFeldman, processors)
}

func TestJointFeldmanUnhappyPath(t *testing.T) {
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
	dkgCommonTest(t, JointFeldman, processors)
}
