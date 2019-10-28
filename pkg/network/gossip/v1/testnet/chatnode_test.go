package testnet

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	gnode "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"
	"github.com/stretchr/testify/assert"
)

// TestChatNodeAsync initializes chatNodes and sends messages using async gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestChatNodeAsync(t *testing.T) {
	assert := assert.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// static fanout set size for each node
	fanoutSize := 5

	chatNodes := make([](*chatNode), numNodes)
	gNodes := make([](*gnode.Node), numNodes)



	// create chatNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		cn, err := newChatNode()
		assert.Nil(err, "chatNode initialization issue")
		chatNodes[i] = cn

		gn, err := cn.startNode(defaultLogger, fanoutSize, numNodes, 50000)
		assert.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		payloadBytes, err := createMsg(fmt.Sprintf("Hello from node %v", i), fmt.Sprintf("node %v", i))
		if err != nil {
			log.Fatalf("could not create message payload: %v", err)
		}

		_, err = gn.AsyncGossip(context.Background(), payloadBytes, nil, "DisplayMessage")
		assert.Nil(err, "at index %v", i)
	}

	// Giving async calls time to finish
	time.Sleep(2 * time.Second)

	// confirm that all messages are received
	for i, cn := range chatNodes {
		// Confirm the count of the messages
		if len(cn.messages) != (numNodes - 1) {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes-1, len(cn.messages))

			// Inform the user which messages are missing
			receivedIndices, err := extractSenderId(numNodes, cn.messages, "Hello from node", 16)
			if err != nil {
				t.Errorf("Unexpected error in extractIndices in node %v. Cannot check message content. Error: %v", i, err)
			}

			for j := 0; j < numNodes; j++ {
				// Nodes don't receive messages from themselves
				if j == i {
					continue
				}
				// If message not received report it.
				if !(*receivedIndices)[j] {
					t.Errorf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", i, j)
				}
			}
		}
	}
}

// TestChatNodeSync initializes chatNodes and sends messages using sync gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestChatNodeSync(t *testing.T) {
	assert := assert.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// fanoutSet size for each node
	fanoutSize := 5

	chatNodes := make([](*chatNode), numNodes)
	gNodes := make([](*gnode.Node), numNodes)


	// create chatNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		cn, err := newChatNode()
		assert.Nil(err, "chatNode initialization issue")
		chatNodes[i] = cn

		gn, err := cn.startNode(defaultLogger.With().Str("node", fmt.Sprintf("%v", i)).Logger(), fanoutSize, numNodes, 51000)
		assert.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		payloadBytes, err := createMsg(fmt.Sprintf("Hello from node %v", i), fmt.Sprintf("node %v", i))
		if err != nil {
			log.Fatalf("could not create message payload: %v", err)
		}

		_, err = gn.SyncGossip(context.Background(), payloadBytes, nil, "DisplayMessage")
		assert.Nil(err, "at index %v", i)
	}

	// Giving sync calls time to finish propagating
	time.Sleep(2 * time.Second)

	// confirm that all messages are received
	for i, cn := range chatNodes {
		// Confirm the count of the messages
		if len(cn.messages) != (numNodes - 1) {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes-1, len(cn.messages))

			// Inform the user which messages are missing
			receivedIndices, err := extractSenderId(numNodes, cn.messages, "Hello from node", 16)
			if err != nil {
				t.Errorf("Unexpected error in extractIndices in node %v. Cannot check message content. Error: %v", i, err)
			}

			for j := 0; j < numNodes; j++ {
				// Nodes don't receive messages from themselves
				if j == i {
					continue
				}
				// If message not received report it.
				if !(*receivedIndices)[j] {
					t.Errorf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", i, j)
				}
			}
		}
	}
}
