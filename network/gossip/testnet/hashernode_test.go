package testnet

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/network/gossip"
)

// TestHasherNodeAsync initializes hasherNodes and sends messages using async gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestHasherNodeAsync(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken hashernode test")

	assert := assert.New(t)
	require := require.New(t)

	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// fanoutSet size for each node
	fanoutSize := 5

	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	require.Nil(err, "could not create hasher")

	// hashes keeps the hash of all the messages sent
	hashes := make([]string, numNodes)

	// keeping track of hasherNodes and their respective gnodes
	hasherNodes := make([](*hasherNode), numNodes)
	gNodes := make([](*gossip.Node), numNodes)

	// create hashedNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		hn, err := newHasherNode()
		assert.Nil(err, "HasherNode initialization issue")
		hasherNodes[i] = hn

		gn, err := hn.startNode(defaultLogger, fanoutSize, numNodes, 48000)
		assert.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		message := fmt.Sprintf("hash %v", i)

		hash := hasher.ComputeHash([]byte(message))
		hashes = append(hashes, string(hash[:]))
		// store in the hasherNode that is sending out the message
		hasherNodes[i].store([]byte(message))

		_, err := gn.AsyncGossip(context.Background(), []byte(message), nil, "Receive")
		assert.Nil(err, "at index %v", i)
	}

	// Giving async calls time to finish
	time.Sleep(2 * time.Second)

	// confirm that all messages are received
	for i, hn := range hasherNodes {
		// Confirm the count of the messages
		if len(hn.messages) != numNodes {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes-1, len(hn.messages))

			// Inform the user which messages are missing
			receivedIndices, err := extractHashIndices(hashes, hn.messages)
			if err != nil {
				t.Errorf("Unexpected error in extractIndices in node %v. Cannot check message content. Error: %v", i, err)
			}

			for j := 0; j < numNodes; j++ {
				// If message not received report it.
				if !(*receivedIndices)[j] {
					t.Errorf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", i, j)
				}
			}
		}
	}
}

// TestHasherNodeSync initializes hasherNode and sends messages using sync gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestHasherNodeSync(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken hashernode test")

	assert := assert.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// fanoutSet size for each node
	fanoutSize := 5

	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	assert.Nil(err, "could not create hasher")

	// hashes keeps the hash of all the messages sent
	hashes := make([]string, numNodes)

	// keeping track of hasherNodes and their respective gnodes
	hasherNodes := make([](*hasherNode), numNodes)
	gNodes := make([](*gossip.Node), numNodes)

	// create hashNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		hn, err := newHasherNode()
		assert.Nil(err, "HasherNode initialization issue")
		hasherNodes[i] = hn

		gn, err := hn.startNode(defaultLogger, fanoutSize, numNodes, 49000)
		assert.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		message := fmt.Sprintf("hash %v", i)

		hash := hasher.ComputeHash([]byte(message))
		hashes = append(hashes, string(hash[:]))
		// store in the hasherNode that is sending out the message
		hasherNodes[i].store([]byte(message))

		_, err := gn.SyncGossip(context.Background(), []byte(message), nil, "Receive")
		assert.Nil(err, "at index: %v", i)
	}

	// Giving async calls time to finish
	time.Sleep(2 * time.Second)

	// confirm that all messages are received
	for i, hn := range hasherNodes {
		// Confirm the count of the messages
		if len(hn.messages) != numNodes {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes-1, len(hn.messages))

			// Inform the user which messages are missing
			receivedIndices, err := extractHashIndices(hashes, hn.messages)
			if err != nil {
				t.Errorf("Unexpected error in extractIndices in node %v. Cannot check message content. Error: %v", i, err)
			}

			for j := 0; j < numNodes; j++ {
				// If message not received report it.
				if !(*receivedIndices)[j] {
					t.Errorf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", i, j)
				}
			}
		}
	}
}
