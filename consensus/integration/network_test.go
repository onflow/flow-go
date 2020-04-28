package integration_test

import (
	"os"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

func connect(nodes []*Node, blockOrDelay BlockOrDelayFunc) {
	nodeDict := make(map[flow.Identifier]*Node, len(nodes))
	for _, n := range nodes {
		nodeDict[n.id.ID()] = n
	}

	for _, n := range nodes {
		{
			sender := n
			submit := func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
				for _, targetID := range targetIDs {
					// find receiver
					receiver, found := nodeDict[targetID]
					if !found {
						continue
					}

					blocked, delay := blockOrDelay(channelID, event, sender, receiver)
					if blocked {
						continue
					}

					go receiveWithDelay(channelID, event, sender, receiver, delay)
				}

				return nil
			}
			n.net.WithSubmit(submit)
		}
	}
}

func receiveWithDelay(channelID uint8, event interface{}, sender, receiver *Node, delay time.Duration) {
	// delay the message sending
	if delay > 0 {
		time.Sleep(delay)
	}

	// statics
	receiver.Lock()
	switch event.(type) {
	case *messages.BlockProposal:
		// add some delay to make sure slow nodes can catch up
		time.Sleep(3 * time.Millisecond)
		receiver.blockproposal++
	case *messages.BlockVote:
		receiver.blockvote++
	case *messages.SyncRequest:
		receiver.syncreq++
	case *messages.SyncResponse:
		receiver.syncresp++
	case *messages.RangeRequest:
		receiver.rangereq++
	case *messages.BatchRequest:
		receiver.batchreq++
	case *messages.BlockResponse:
		receiver.batchresp++
	default:
		panic("received message")
	}
	receiver.Unlock()

	// find receiver engine
	receiverEngine := receiver.net.engines[channelID]

	// give it to receiver engine
	receiverEngine.Submit(sender.id.ID(), event)
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		n.db.Close()
		os.RemoveAll(n.dbDir)
	}
}
