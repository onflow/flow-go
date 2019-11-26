package testnet

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatNodeOneToAll initializes chatNodes and sends messages using gossip from each node to
// every other node, checking whether the nodes are correctly received
func TestChatNodeOneToAll(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken chatnode test")

	assert := assert.New(t)
	require := require.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// static fanout set size for each node
	fanoutSize := 5
	wg := &sync.WaitGroup{}

	chatNodes := make([](*chatNode), numNodes)
	gNodes := make([](*gossip.Node), numNodes)
	listeners, availableAddresses := FindPorts(numNodes)

	// create chatNodes and initialize gNodes with them and add them to the slices
	for i := 0; i < numNodes; i++ {
		cn, err := newChatNode(wg)
		require.Nil(err, "chatNode initialization issue")
		chatNodes[i] = cn

		gn, err := cn.startNode(defaultLogger, fanoutSize, numNodes, i, listeners, availableAddresses)
		require.Nil(err, "gnode initialization issue")
		gNodes[i] = gn
	}

	// send the messages to one another
	for i, gn := range gNodes {
		payloadBytes, err := createMsg(fmt.Sprintf("Hello from node %v", i), fmt.Sprintf("node %v", i))
		require.Nil(err, fmt.Sprintf("could not create message payload: %v", err))

		wg.Add(numNodes - 1)
		_, err = gn.Gossip(context.Background(), payloadBytes, nil, DisplayMessage)
		require.Nil(err, "at index %v, Errors: %v", i, err)
	}

	// Giving gossip time to finish propagating
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
	for i, cn := range chatNodes {
		// Confirm the count of the messages
		if len(cn.messages) != (numNodes - 1) {
			t.Errorf("Node #%v has wrong number of messages. Expected: %v, Got: %v", i, numNodes-1, len(cn.messages))

			// Inform the user which messages are missing
			receivedIndices, err := extractSenderID(numNodes, cn.messages, "Hello from node", 16)
			if err != nil {
				t.Errorf("Unexpected error in extractIndices in node %v. Cannot check message content. Error: %v", i, err)
			}

			for j := 0; j < numNodes; j++ {
				// Nodes don't receive messages from themselves
				if j == i {
					assert.False((*receivedIndices)[i], fmt.Sprintf("self gossiped for node %v detected", i))
				}
				// If message not received report it.
				if !(*receivedIndices)[j] {
					t.Errorf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", i, j)
				}
			}
		}
	}
	for _, ln := range listeners {
		assert.False(ln.Close() != nil, "Error closing the listeners")
	}
}

// TestOnlyRecipientChatNodes initializes chatNodes and sends messages using gossip from each node to
// to a subset of node, checking whether the messages are receiving by and only by the recipients
func TestOnlyRecipientChatNodes(t *testing.T) {
	// TODO: Fix broken test
	t.Skip("skipping broken chatnode test")

	assert := assert.New(t)
	require := require.New(t)
	// number of nodes to initialize, to add more, increase the size of the portPool in the initializer
	numNodes := 10
	// static fanout set size for each node
	fanoutSize := 5
	wg := &sync.WaitGroup{}

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
		chatNodes := make([](*chatNode), numNodes)
		gNodes := make([](*gossip.Node), numNodes)
		listeners, availableAddresses := FindPorts(numNodes)

		// create chatNodes and initialize gNodes with them and add them to the slices
		for i := 0; i < numNodes; i++ {
			cn, err := newChatNode(wg)
			require.Nil(err, "chatNode initialization issue")
			chatNodes[i] = cn

			gn, err := cn.startNode(defaultLogger, fanoutSize, numNodes, i, listeners, availableAddresses)
			require.Nil(err, "gnode initialization issue")
			gNodes[i] = gn
		}

		// make each node send a message and confirm it is received by the correct nodes
		for i := 0; i < numNodes; i++ {
			// select a subset of size numRecipients from the address pool
			indices, addresses := randomSubset(tc.numRecipients, availableAddresses)
			numRecipients := tc.numRecipients
			// exclude the node itself if it is chosen
			if indices[i] {
				indices[i] = false
				numRecipients--
			}

			//create the message to be sent
			msgString := fmt.Sprintf("Hello from node %v", i)
			payloadBytes, err := createMsg(msgString, fmt.Sprintf("node %v", i))
			require.Nil(err, "count not create message")

			wg.Add(numRecipients)
			//send the message to the selected recipients by OneToMany
			_, err = gNodes[i].Gossip(context.Background(), payloadBytes, addresses, DisplayMessage)
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
				msgFound := contains(chatNodes[j].messages, msgString)
				// if j is one of the indices, then the chatNode at index j should have received the message
				if indices[j] {
					assert.True(msgFound, fmt.Sprintf("node %v should contain a message from node %v", j, i))
				} else {
					// otherwise, it should not have received the message
					assert.False(msgFound, fmt.Sprintf("node %v should not contain a message from node %v", j, i))
				}
				msgFoundDuplicate := containsDuplicate(chatNodes[j].messages, msgString)
				assert.False(msgFoundDuplicate, fmt.Sprintf("node %v contains duplicate values. there should be no duplicate values", j))
			}
		}
		//close the listeners after use
		for _, ln := range listeners {
			assert.False(ln.Close() != nil, "Error closing the listeners")
		}
	}
}
