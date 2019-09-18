package crypto

import (
	"fmt"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

//type dkgChan chan []byte
type dkgChan chan DKGmsg

func send(network [][]dkgChan, orig int, dest int, msg DKGmsg) {
	fmt.Println("Sending msg", orig, dest, msg)
	network[orig][dest] <- msg
}

func broadcast(network [][]dkgChan, orig int, msg DKGmsg) {
	fmt.Println("Broadcasting msg", orig, msg)
	for i := 0; i < len(network[orig]); i++ {
		if i != orig {
			network[orig][i] <- msg
		}
	}
}

func (out *DKGoutput) processOutput(current int, network [][]dkgChan) dkgResult {
	if out.err != nil {
		log.Error("DKG output error: " + out.err.Error())
		return out.result
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
	log.Debug("Feldman VSS starts")
	// number of nodes to test
	n := 4
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
			dkg, err := NewDKG(FeldmanVSS, n, current, lead)
			if err != nil {
				log.Error(err.Error())
				return
			}
			out := dkg.StartDKG()
			_ = out.processOutput(current, network)

			// the current node listens continuously
			orig := make([]int, n-1)
			for j := 0; j < n-1; j++ {
				orig[j] = index(current, j)
			}
			for {
				select {
				case msg := <-network[orig[0]][current]:
					out = dkg.ProcessDKGmsg(orig[0], msg)
					_ = out.processOutput(current, network)
				case msg := <-network[orig[1]][current]:
					out = dkg.ProcessDKGmsg(orig[1], msg)
					_ = out.processOutput(current, network)
				case msg := <-network[orig[2]][current]:
					out = dkg.ProcessDKGmsg(orig[2], msg)
					_ = out.processOutput(current, network)
				// if timeout, stop and finalize
				case <-time.After(time.Second):
					_, _, _, _ = dkg.EndDKG()
					fmt.Printf("quit %d \n", current)
					quit <- 1
					return
				}
			}
			// first time out and second timeout ?
		}(current)
	}

	// this loop avoids ending the main thread
	for i := 0; i < n; i++ {
		<-quit
	}
}
