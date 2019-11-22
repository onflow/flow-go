package testnet

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// helper contains helper functions and constants used in tests_test

var (
	defaultLogger = zerolog.New(ioutil.Discard)
	timeout       = 200 * time.Millisecond
)

// FindPorts goes and finds num different empty ports and returns a slice of listeners to these ports as well as their addresses
func FindPorts(num int) (listeners []net.Listener, addresses []string) {
	for num > 0 {
		//generate random port, range: 1000-61000
		port := 1000 + rand.Intn(60000)
		address := fmt.Sprintf("127.0.0.1:%v", port)

		ln, err := net.Listen("tcp", address)
		if err != nil {
			continue
		}
		num--
		listeners = append(listeners, ln)
		addresses = append(addresses, address)
	}

	return
}

// extractSenderId returns a bool array with the index i true if there is a message from node i in the provided messages.
func extractSenderId(numNodes int, messages []string, expectedMsgTxt string, expectedMsgSize int) (*[]bool, error) {
	indices := make([]bool, numNodes)
	for _, msg := range messages {
		if len(msg) < expectedMsgSize {
			return nil, fmt.Errorf("invalid message format")
		}
		nodeID, err := strconv.Atoi(msg[expectedMsgSize:])
		if err != nil {
			return nil, fmt.Errorf("could not extract the node id from: %v", msg)
		}

		if indices[nodeID] {
			return nil, fmt.Errorf("duplicate message reception: %v", msg)
		}

		if msg == fmt.Sprintf("%s %v", expectedMsgTxt, nodeID) {
			indices[nodeID] = true
		}
	}
	return &indices, nil
}

// extractHashIndices returns a bool array with the index i true if there is a message from node i in the provided messages.
// allHashes: the hashes of all the messages sent out. containedHashes: The hashes of the messages received and stored by the hasherNode
func extractHashIndices(allHashes []string, containedHashes []string) (*[]bool, error) {
	indices := make([]bool, len(allHashes))
	// loop over all the hashes inside the hasherNode
	for _, containedHash := range containedHashes {
		// compare them to the all the hashes sent out
		for i, hash := range allHashes {
			if hash == containedHash {
				indices[i] = true
				break
			}
		}
	}
	return &indices, nil
}

// randomSubset returns a random subset of size n from a range of addresses. It returns the
// indices of the addresses (starting from the startPort) as well as the addresses themselves
func randomSubset(size int, availableAddresses []string) (indices map[int]bool, addressesSubset []string) {
	numNodesTotal := len(availableAddresses)
	if size > numNodesTotal || size < 0 {
		return nil, nil
	}
	indices = make(map[int]bool)

	for {
		if len(indices) == size {
			return
		}
		idx := rand.Intn(numNodesTotal)
		if indices[idx] {
			continue
		}
		indices[idx] = true
		addressesSubset = append(addressesSubset, availableAddresses[idx])
	}
}

// contains returns true if a slice contains a specific string
func contains(slice []string, str string) bool {
	for _, el := range slice {
		if el == str {
			return true
		}
	}
	return false
}

// containsDuplicate returns true if a slice contains a specific string more than once
func containsDuplicate(slice []string, str string) bool {
	cnt := 0
	for _, el := range slice {
		if el == str {
			cnt++
		}
	}
	return cnt > 1
}
