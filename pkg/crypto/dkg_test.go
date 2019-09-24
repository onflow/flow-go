package crypto

import (
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

//type dkgChan chan []byte
type dkgChan chan DKGmsg

// maps the interval [0..n-1] into [0..c-1,c+1..n] if c>1
// maps the interval [0..n-1] into [1..n] if c=0
func index(current int, loop int) int {
	if loop < current {
		return loop
	}
	return loop + 1
}

func send(network [][]dkgChan, orig int, dest int, msg DKGmsg) {
	log.Debug(fmt.Sprintf("%d Sending msg to %d:\n", orig, dest))
	log.Debug(msg)
	network[orig][dest] <- msg
}

func broadcast(network [][]dkgChan, orig int, msg DKGmsg) {
	log.Debug(fmt.Sprintf("%d Broadcasting:", orig))
	log.Debug(msg)
	for i := 0; i < len(network[orig]); i++ {
		if i != orig {
			network[orig][i] <- msg
		}
	}
}

func (out *DKGoutput) processOutput(current int, network [][]dkgChan) dkgResult {
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
		return nonApplicable
	}

	for _, msg := range out.action {
		if msg.broadcast {
			broadcast(network, current, msg.data)
		} else {
			send(network, current, msg.dest, msg.data)
		}
	}
	return out.result
}

func TestFeldmanVSS(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 3
	lead := 0

	// Create channels
	quit := make(chan int)
	network := make([][]dkgChan, n)
	for i := 0; i < n; i++ {
		network[i] = make([]dkgChan, n)
		for j := 0; j < n; j++ {
			network[i][j] = make(dkgChan)
		}
	}

	for current := 0; current < n; current++ {
		go func(current int) {
			g := gomega.NewWithT(t)
			dkg, err := NewDKG(FeldmanVSS, n, current, lead)
			if err != nil {
				log.Error(err.Error())
				return
			}
			out := dkg.StartDKG()
			res := out.processOutput(current, network)
			g.Expect(res).To(Equal(nonApplicable))

			// the current node listens continuously
			orig := make([]int, n-1)
			for j := 0; j < n-1; j++ {
				orig[j] = index(current, j)
			}
			for {
				select {
				case msg := <-network[orig[0]][current]:
					out = dkg.ProcessDKGmsg(orig[0], msg)
					res := out.processOutput(current, network)
					g.Expect(res).To(Equal(valid))
				case msg := <-network[orig[1]][current]:
					out = dkg.ProcessDKGmsg(orig[1], msg)
					res := out.processOutput(current, network)
					g.Expect(res).To(Equal(valid))
					/*case msg := <-network[orig[2]][current]:
					out = dkg.ProcessDKGmsg(orig[2], msg)
					out.processOutput(current, network)*/
				// if timeout, stop and finalize
				case <-time.After(time.Second):
					_, _, _, _ = dkg.EndDKG()
					log.Debug(fmt.Sprintf("%d quit \n", current))
					quit <- 1
					return
				}
			}
		}(current)
	}

	// this loop avoids ending the main thread
	for i := 0; i < n; i++ {
		<-quit
	}
}
