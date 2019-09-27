package crypto

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// maps the interval [0..n-1] into [0..c-1,c+1..n] if c>1
// maps the interval [0..n-1] into [1..n] if c=0
func index(current int, loop int) int {
	if loop < current {
		return loop
	}
	return loop + 1
}

type toProcess struct {
	orig int
	msg  DKGmsg
}

func send(orig int, dest int, msg DKGmsg, dkg []DKGstate, chans []chan *toProcess) {
	log.Info(fmt.Sprintf("%d Sending to %d:\n", orig, dest))
	log.Debug(msg)
	new := &toProcess{orig, msg}
	chans[dest] <- new
}

func broadcast(orig int, dkg []DKGstate, msg DKGmsg, chans []chan *toProcess) {
	log.Info(fmt.Sprintf("%d Broadcasting:", orig))
	log.Debug(msg)
	new := &toProcess{orig, msg}
	for i := 0; i < len(dkg); i++ {
		if i != orig {
			chans[i] <- new
		}
	}
}

func processChan(current int, dkg []DKGstate, chans []chan *toProcess, quit chan int) {
	for {
		select {
		case new := <-chans[current]:
			log.Info(fmt.Sprintf("%d Receiving from %d:", current, new.orig))
			out := dkg[current].ProcessDKGmsg(new.orig, new.msg)
			out.processOutput(current, dkg, chans)
		// if timeout, stop and finalize
		case <-time.After(time.Second):
			log.Info(fmt.Sprintf("%d quit \n", current))
			_, _, _, _ = dkg[current].EndDKG()
			quit <- 1
			return
		}
	}
}

func (out *DKGoutput) processOutput(current int, dkg []DKGstate,
	chans []chan *toProcess) {
	assert.Nil(x, out.err)
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
	}

	assert.Equal(x, out.result, valid, "they should be equal")

	for _, msg := range out.action {
		if msg.broadcast {
			broadcast(current, dkg, msg.data, chans)
		} else {
			send(current, msg.dest, msg.data, dkg, chans)
		}
	}
}

var x *testing.T

func TestFeldmanVSS(t *testing.T) {
	x = t
	log.SetLevel(log.ErrorLevel)
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 5
	lead := 0
	dkg := make([]DKGstate, n)
	quit := make(chan int)
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
	// create the node channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 10)
		go processChan(i, dkg, chans, quit)
	}
	// start DKG in all nodes but the leader
	for current := 0; current < n; current++ {
		if current != lead {
			out := dkg[current].StartDKG()
			out.processOutput(current, dkg, chans)
		}
	}
	// start the leader (this avoids a data racing issue)
	out := dkg[lead].StartDKG()
	out.processOutput(lead, dkg, chans)

	// this loop synchronizes the main thread to end all DKGs
	for i := 0; i < n; i++ {
		<-quit
	}
}
