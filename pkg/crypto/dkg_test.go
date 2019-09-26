package crypto

import (
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
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

func processChan(current int, dkg []DKGstate, chans []chan *toProcess, quit chan int, g []*WithT) {
	//g := gomega.NewWithT(t)
	for {
		select {
		case new := <-chans[current]:
			log.Info(fmt.Sprintf("%d Receiving from %d:", current, new.orig))
			out := dkg[current].ProcessDKGmsg(new.orig, new.msg)
			out.processOutput(current, dkg, chans, g[current])
		// if timeout, stop and finalize
		case <-time.After(time.Second):
			quit <- 1
			return
		}
	}
}

func (out *DKGoutput) processOutput(current int, dkg []DKGstate, chans []chan *toProcess, g *WithT) {
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
	}

	g.Expect(out.result).To(Equal(valid))

	for _, msg := range out.action {
		if msg.broadcast {
			broadcast(current, dkg, msg.data, chans)
		} else {
			send(current, msg.dest, msg.data, dkg, chans)
		}
	}
}

func TestFeldmanVSSSimple(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 5
	lead := 0
	dkg := make([]DKGstate, n)
	quit := make(chan int)
	chans := make([]chan *toProcess, n)
	g := make([]*WithT, n)
	for i := 0; i < n; i++ {
		g[i] = gomega.NewWithT(t)
	}

	// create DKG in all nodes
	for current := 0; current < n; current++ {
		var err error
		dkg[current], err = NewDKG(FeldmanVSS, n, current, lead)
		if err != nil { // change to g[i]
			log.Error(err.Error())
			return
		}
	}

	// create the nodes channels
	for i := 0; i < n; i++ {
		chans[i] = make(chan *toProcess, 10)
		go processChan(i, dkg, chans, quit, g)
	}
	// start DKG in all nodes but leader
	for current := 0; current < n; current++ {
		if current != lead {
			out := dkg[current].StartDKG()
			out.processOutput(current, dkg, chans, g[current])
		}
	}
	// start the leader (this avoids a data racing issue)
	out := dkg[lead].StartDKG()
	out.processOutput(lead, dkg, chans, g[lead])

	// this loop synchronizes the main thread to end all DKGs
	for i := 0; i < n; i++ {
		<-quit
	}
	// close all DKGs
	for i := 0; i < n; i++ {
		_, _, _, _ = dkg[i].EndDKG()
		log.Info(fmt.Sprintf("%d quit \n", i))
	}
}
