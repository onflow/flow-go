package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/model/flow"
)

type RcvQueueTestSuite struct {
	suite.Suite
	q    *RcvQueue
}

// Helper function to create a message
func createMessage() *message.Message {
	var ids []flow.Identifier

	for i := 0; i < 2; i++ {
		// generating ids of the nodes
		// as [32]byte{(i+1),0,...,0}
		var target [32]byte
		target[0] = byte(i + 1)
		targetID := flow.Identifier(target)
		ids = append(ids, targetID)
	}

	return &message.Message {
		ChannelID: 1,
		EventID:   []byte("1"),
		OriginID:  ids[0][:],
		TargetIDs: [][]byte{ids[1][:]},
		Payload:   []byte(`{"Code":21,"Data":{"Nonce":0,"EntityIDs":[],"Blobs":[[]]}}`),
	}
}

func TestRcvQueueTestSuite(t *testing.T) {
	suite.Run(t, new(RcvQueueTestSuite))
}

// SetupTest creates a new queue
func (r *RcvQueueTestSuite) SetupTest() {
	q, err := NewRcvQueue(zerolog.New(os.Stderr), 10)
	require.NoError(r.Suite.T(), err)
	engines := make(map[uint8]network.Engine)
	q.SetEngines(&engines)

	r.q = q
}

// TestPriorityScore submits a decoded message to calculate a priority score by message type
func (r *RcvQueueTestSuite) TestPriorityScore() {
	msg := createMessage()

	payload, payloadErr := r.q.codec.Decode(msg.Payload)
	score := r.q.Priority(payload)

	assert.True(r.Suite.T(), payloadErr == nil)
	assert.True(r.Suite.T(), score == 3)
}

// TestSizeScore sends a message size to calculate a size score by message size
func (r *RcvQueueTestSuite) TestSizeScore() {
	assert.True(r.Suite.T(), r.q.Size(34567) == 2)
}

// TestScore submits a message to calculate a total score
func (r *RcvQueueTestSuite) TestScore() {
	msg := createMessage()
	payload, payloadErr := r.q.codec.Decode(msg.Payload)
	score := r.q.Score(msg.Size(), payload)

	assert.True(r.Suite.T(), payloadErr == nil)
	assert.True(r.Suite.T(), score == 6)
}

// TestSingleMessageAdd adds a single message to the queue and verifies its existence
func (r *RcvQueueTestSuite) TestSingleMessageAdd() {
	msg := createMessage()

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	added := r.q.Add(senderID, msg)

	assert.True(r.Suite.T(), added)
	assert.True(r.Suite.T(), r.q.workers == 1)
	assert.True(r.Suite.T(), r.q.cache.Len() == 1)
	assert.True(r.Suite.T(), r.q.queue.Len() == 1)
	time.Sleep(time.Second)
	assert.True(r.Suite.T(), r.q.workers == 0)
	assert.True(r.Suite.T(), r.q.queue.Len() == 0)
	assert.True(r.Suite.T(), r.q.stacks[5].Len() == 0)
}

// TestDuplicateMessageAdd adds a single message twice to the queue and verifies its rejection
func (r *RcvQueueTestSuite) TestDuplicateMessageAdd() {
	msg := createMessage()

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	r.q.Add(senderID, msg)
	added := r.q.Add(senderID, msg)

	assert.False(r.Suite.T(), added)
}
