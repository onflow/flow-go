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

func sendChannels(network [][]dkgChan, orig int, dest int, msg DKGmsg) {
	log.Info(fmt.Sprintf("%d Sending msg to %d:\n", orig, dest))
	log.Debug(msg)
	network[orig][dest] <- msg
}

func broadcastChannels(network [][]dkgChan, orig int, msg DKGmsg) {
	log.Info(fmt.Sprintf("%d Broadcasting:", orig))
	log.Debug(msg)
	for i := 0; i < len(network[orig]); i++ {
		if i != orig {
			network[orig][i] <- msg
		}
	}
}

func (out *DKGoutput) processOutputChannels(current int, network [][]dkgChan) dkgResult {
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
		return valid
	}

	for _, msg := range out.action {
		if msg.broadcast {
			broadcastChannels(network, current, msg.data)
		} else {
			sendChannels(network, current, msg.dest, msg.data)
		}
	}
	return out.result
}

func TestFeldmanVSSChannels(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 5
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
				log.Info(fmt.Sprintf("%d quit \n", current))
				quit <- 1
				return
			}
			out := dkg.StartDKG()
			res := out.processOutputChannels(current, network)
			g.Expect(res).To(Equal(valid))

			// the current node listens continuously
			orig := make([]int, n-1)
			for j := 0; j < n-1; j++ {
				orig[j] = index(current, j)
			}
			for {
				select {
				case msg := <-network[orig[0]][current]:
					out = dkg.ProcessDKGmsg(orig[0], msg)
					res := out.processOutputChannels(current, network)
					g.Expect(res).To(Equal(valid))
					//g.Eventually(func() dkgResult { return res }).Should(Equal(valid))
				case msg := <-network[orig[1]][current]:
					out = dkg.ProcessDKGmsg(orig[1], msg)
					_ = out.processOutputChannels(current, network)
					//g.Expect(res).To(Equal(valid))
				case msg := <-network[orig[2]][current]:
					out = dkg.ProcessDKGmsg(orig[2], msg)
					_ = out.processOutputChannels(current, network)
					//g.Expect(res).To(Equal(valid))
				case msg := <-network[orig[3]][current]:
					out = dkg.ProcessDKGmsg(orig[3], msg)
					_ = out.processOutputChannels(current, network)
					//g.Expect(res).To(Equal(valid))
				// if timeout, stop and finalize
				case <-time.After(time.Second):
					_, _, _, _ = dkg.EndDKG()
					log.Info(fmt.Sprintf("%d quit \n", current))
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

func send(orig int, dest int, msg DKGmsg, dkg []DKGstate) {
	log.Info(fmt.Sprintf("%d Sending msg to %d:\n", orig, dest))
	log.Info(msg)
	go func() {
		out := dkg[dest].ProcessDKGmsg(orig, msg)
		Expect(out.result).To(Equal(valid))
		out.processOutput(dest, dkg)
	}()
}

func broadcast(orig int, dkg []DKGstate, msg DKGmsg) {
	log.Info(fmt.Sprintf("%d Broadcasting:", orig))
	log.Debug(msg)
	for i := 0; i < len(dkg); i++ {
		if i != orig {
			go func(i int) {
				out := dkg[i].ProcessDKGmsg(orig, msg)
				Expect(out.result).To(Equal(valid))
				out.processOutput(i, dkg)
			}(i)
		}
	}
}

func (out *DKGoutput) processOutput(current int, dkg []DKGstate) {
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
	}

	Expect(out.result).To(Equal(valid))

	for _, msg := range out.action {
		if msg.broadcast {
			broadcast(current, dkg, msg.data)
		} else {
			send(current, msg.dest, msg.data, dkg)
		}
	}
}

func TestFeldmanVSSSimple(t *testing.T) {
	log.SetLevel(log.InfoLevel)
	log.Debug("Feldman VSS starts")
	RegisterTestingT(t)
	// number of nodes to test
	n := 5
	lead := 0
	dkg := make([]DKGstate, n)

	for current := 0; current < n; current++ {
		var err error
		dkg[current], err = NewDKG(FeldmanVSS, n, current, lead)

		if err != nil {
			log.Error(err.Error())
			return
		}
		// start the DKG in all nodes but the leader
		if current != lead {
			go func(current int) {
				out := dkg[current].StartDKG()
				out.processOutput(current, dkg)
			}(current)
		}
	}

	// start the leader
	go func() {
		out := dkg[lead].StartDKG()
		out.processOutput(lead, dkg)
	}()

	time.Sleep(time.Second)
	// end all nodes
	for current := 0; current < n; current++ {
		_, _, _, _ = dkg[current].EndDKG()
	}

}
