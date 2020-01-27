package testnet

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHasherNodeOneToAll initializes hasherNodes and sends messages using gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestHasherNodeOneToAll(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken hashernode test")

	assert := assert.New(t)
	require := require.New(t)
	wg := &sync.WaitGroup{}
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// fanoutSet size for each node
	fanoutSize := 5

	hasher, err := crypto.NewSha3_256()
	require.Nil(err, "could not create hasher")

	// hashes keeps the hash of all the messages sent
	hashes := make([]string, numNodes)

	// keeping track of hasherNodes and their respective gnodes
	hasherNodes := make([](*hasherNode), numNodes)
	gNodes := make([](*gossip.Node), numNodes)
	listeners, availableAddresses := FindPorts(numNodes)

	// create hashedNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		hn, err := newHasherNode(wg)
		assert.Nil(err, "HasherNode initialization issue")
		hasherNodes[i] = hn

		gn, err := hn.startNode(defaultLogger, fanoutSize, numNodes, i, listeners, availableAddresses)
		assert.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		message := fmt.Sprintf("hash %v", i)

		hash := hasher.ComputeHash([]byte(message))
		hashes[i] = string(hash[:])
		// store in the hasherNode that is sending out the message
		hasherNodes[i].store([]byte(message))

		//Number of nodes that should receive this message
		wg.Add(numNodes - 1)
		_, err = gn.Gossip(context.Background(), []byte(message), nil, Receive)

		assert.Nil(err, "at index %v", i)
	}

	// Giving gossip calls time to finish propagating
	waitChannel := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChannel)
	}()
	select {
	case <-waitChannel:
	case <-time.After(timeout):
		t.Logf("Test timed out")
	}

	// confirm that all messages are received
	for i, hn := range hasherNodes {
		// Confirm the count of the messages
		if len(hn.messages) != numNodes {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes, len(hn.messages))

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
			t.Errorf("Node messages: %v", hn.messages)
		}
	}
	for _, ln := range listeners {
		ln.Close()
	}
}

// TestHasherNode initializes hasherNodes and sends messages using gossip from each node to
// a subset of other nodes, checking whether the messages are correctly received by only the correct recipients
func TestHasherNode(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken hashernode test")

	assert := assert.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// fanoutSet size for each node
	fanoutSize := 5
	wg := &sync.WaitGroup{}

	hasher := crypto.NewSha3_256()
	assert.Nil(err, "could not create hasher")

	testParams := []struct {
		numRecipients int
		sync          bool
	}{
		{
			numRecipients: 3,    //oneToMany
			sync:          true, //sync gossip
		},
		//{ //broken test
		//	numRecipients: 7, //oneToMany with more recipients than fanout size, resulting in dynamic fanout being used
		//	sync:          true,
		//},
		{
			numRecipients: 1, //oneToOne
			sync:          true,
		},
		{
			numRecipients: 3,     //oneToMany
			sync:          false, //async gossip
		},
		{
			numRecipients: 7, //oneToMany with more recipients than fanout size, resulting in dynamic fanout being used
			sync:          false,
		},
		{
			numRecipients: 1, //oneToOne
			sync:          false,
		},
	}

	for _, tc := range testParams {

		// keeping track of hasherNodes and their respective gnodes
		hasherNodes := make([](*hasherNode), numNodes)
		gNodes := make([](*gossip.Node), numNodes)
		listeners, availableAddresses := FindPorts(numNodes)

		// create hashNodes and initialize gNodes with them and add them to the slices
		for i := 0; i < numNodes; i++ {
			hn, err := newHasherNode(wg)
			assert.Nil(err, "HasherNode initialization issue")
			hasherNodes[i] = hn

			gn, err := hn.startNode(defaultLogger, fanoutSize, numNodes, i, listeners, availableAddresses)
			assert.Nil(err, "gnode initialization issue")
			gNodes[i] = gn
		}

		// make each node send a message and confirm it is received by the correct nodes
		for i := 0; i < numNodes; i++ {
			// select a subset of size numRecipients from the address pool
			indices, addresses := randomSubset(tc.numRecipients, availableAddresses)
			numRecipients := tc.numRecipients
			// set the index of the node itself as false since a node cannot receive its own message
			if indices[i] {
				indices[i] = false
				numRecipients--
			}

			message := fmt.Sprintf("hash %v", i)
			//create the message to be sent
			hash := hasher.ComputeHash([]byte(message))

			//Number of nodes that should receive this message
			wg.Add(numRecipients)
			//send the message to the selected recipients by OneToMany
			_, err = gNodes[i].Gossip(context.Background(), []byte(message), addresses, Receive)
			assert.Nil(err, "at index %v", i)

			// Giving gossip calls time to finish propagating
			waitChannel := make(chan struct{})
			go func() {
				wg.Wait()
				close(waitChannel)
			}()
			select {
			case <-waitChannel:
			case <-time.After(timeout):
				t.Logf("Test timed out")
			}

			for j := 0; j < numNodes; j++ {
				msgFound := contains(hasherNodes[j].messages, string(hash[:]))
				// if j is one of the indices, then the chatNode at index j should have received the message
				if indices[j] {
					assert.True(msgFound, fmt.Sprintf("node %v should contain a message from node %v", j, i))
				} else {
					assert.False(msgFound, fmt.Sprintf("node %v should not contain a message from node %v", j, i))
				}
				msgFoundDuplicate := containsDuplicate(hasherNodes[j].messages, string(hash[:]))
				assert.False(msgFoundDuplicate, fmt.Sprintf("node %v contains duplicate values. there should be no duplicate values", j))
			}
		}
		//close the listeners after use
		for _, ln := range listeners {
			ln.Close()
		}
	}
}
