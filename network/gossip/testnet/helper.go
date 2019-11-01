package testnet

import (
	"fmt"
	"io/ioutil"
	"net"
	"strconv"

	"github.com/rs/zerolog"
)

// helper contains helper functions and constants used in tests_test

var defaultLogger = zerolog.New(ioutil.Discard)

// pickPort picks the first available port from port pool and returns a listener to that port,
// the port itself, and a list containing the remaining ports
func pickPort(portPool []string) (ln net.Listener, myPort string, othersPort []string, err error) {
	othersPort = make([]string, 0)

	for _, port := range portPool {
		ln, err = net.Listen("tcp", port)
		if err == nil {
			myPort = port
			break
		}
	}

	// if ln is nil then no valid port was found
	if ln == nil {
		return nil, "", nil, fmt.Errorf("could not find an empty port in the given pool")
	}

	// listing the port number of other nodes
	for _, port := range portPool {
		if port != myPort {
			othersPort = append(othersPort, port)
		}
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
