package dkg

import (
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// WhiteboardClient is a test implementation of the DKGContractClient interface.
// It uses a shared whiteboard where nodes post/read broadcast messages and
// results.
type WhiteboardClient struct {
	nodeID     flow.Identifier
	whiteboard *whiteboard
}

// NewWhiteboardClient instantiates a new WhiteboardClient with a reference to
// an existing whiteboard object.
func NewWhiteboardClient(nodeID flow.Identifier, whiteboard *whiteboard) *WhiteboardClient {
	return &WhiteboardClient{
		nodeID:     nodeID,
		whiteboard: whiteboard,
	}
}

// Broadcast implements the DKGContractClient interface. It adds a message to
// the whiteboard.
func (wc *WhiteboardClient) Broadcast(msg messages.BroadcastDKGMessage) error {
	wc.whiteboard.post(msg)
	return nil
}

// ReadBroadcast implements the DKGContractClient interface. It reads messages
// from the whiteboard.
func (wc *WhiteboardClient) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.BroadcastDKGMessage, error) {
	msgs := wc.whiteboard.read(fromIndex)
	return msgs, nil
}

// SubmitResult implements the DKGContractClient interface. It publishes the
// DKG results under the node's ID.
func (wc *WhiteboardClient) SubmitResult(groupKey crypto.PublicKey, pubKeys []crypto.PublicKey) error {
	wc.whiteboard.submit(wc.nodeID, groupKey, pubKeys)
	return nil
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// whiteboard is a concurrency-safe object where nodes can publish/read DKG
// broadcast messages, and DKG results. It is used for testing in place of the
// DKG smart-contract. Multiple WhiteboardClient can safely reference and use
// the same whiteboard concurrently.
type whiteboard struct {
	sync.Mutex
	messages          []messages.BroadcastDKGMessage        // the ordered list of broadcast messages as published by clients
	results           map[flow.Identifier]result            // a map of result-hash to result
	resultSubmitters  map[flow.Identifier][]flow.Identifier // a map of result-hash to a list of node IDs that submitted the result
	resultBySubmitter map[flow.Identifier]result            // a map of node IDs to the submitted result
}

type result struct {
	groupKey crypto.PublicKey
	pubKeys  []crypto.PublicKey
}

// Fingerprint implements the Fingerprinter interface used by MakeID
func (r result) Fingerprint() []byte {
	fingerprint := r.groupKey.Encode()
	for _, pk := range r.pubKeys {
		fingerprint = append(fingerprint, pk.Encode()...)
	}
	return fingerprint
}

func newWhiteboard() *whiteboard {
	return &whiteboard{
		results:           make(map[flow.Identifier]result),
		resultSubmitters:  make(map[flow.Identifier][]flow.Identifier),
		resultBySubmitter: make(map[flow.Identifier]result),
	}
}

func (w *whiteboard) post(msg messages.BroadcastDKGMessage) {
	w.Lock()
	defer w.Unlock()
	w.messages = append(w.messages, msg)
}

func (w *whiteboard) read(fromIndex uint) []messages.BroadcastDKGMessage {
	w.Lock()
	defer w.Unlock()
	return w.messages[fromIndex:]
}

func (w *whiteboard) submit(nodeID flow.Identifier, groupKey crypto.PublicKey, pubKeys []crypto.PublicKey) {
	w.Lock()
	defer w.Unlock()

	result := result{groupKey: groupKey, pubKeys: pubKeys}
	resultHash := flow.MakeID(result)

	w.results[resultHash] = result

	signers, _ := w.resultSubmitters[resultHash]
	signers = append(signers, nodeID)
	w.resultSubmitters[resultHash] = signers
	w.resultBySubmitter[nodeID] = result
}
