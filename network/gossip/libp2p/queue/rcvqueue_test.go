package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"os"
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/mocks"
)

type RcvQueueTestSuite struct {
	suite.Suite
	q *RcvQueue
}

// Helper function to create a message
func createMessage(seed int) *message.Message {
	var ids []flow.Identifier

	for i := seed; i < (2 + seed); i++ {
		// generating ids of the nodes
		// as [32]byte{(i+1),0,...,0}
		var target [32]byte
		target[0] = byte(i + 1)
		targetID := flow.Identifier(target)
		ids = append(ids, targetID)
	}

	return &message.Message{
		ChannelID: 1,
		EventID:   []byte(strconv.Itoa(seed)),
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
	q, err := NewRcvQueue(zerolog.New(os.Stderr), 100)
	require.NoError(r.Suite.T(), err)
	r.q = q
}

// TestPriorityScore submits a decoded message to calculate a priority score by message type
func (r *RcvQueueTestSuite) TestPriorityScore() {
	msg := createMessage(0)

	payload, payloadErr := r.q.codec.Decode(msg.Payload)
	score := r.q.Priority(payload)

	assert.NoError(r.Suite.T(), payloadErr)
	// All of our mocked messages should have a priority score of 3
	assert.Equal(r.Suite.T(), 4, score)
}

// TestSizeScore sends a message size to calculate a size score by message size
func (r *RcvQueueTestSuite) TestSizeScore() {
	msg := createMessage(0)

	// All of our mocked messages should have a size score of 10
	assert.Equal(r.Suite.T(), 10, r.q.Size(float64(msg.Size())))
}

// TestScore submits a message to calculate a total score
func (r *RcvQueueTestSuite) TestScore() {
	msg := createMessage(0)

	payload, payloadErr := r.q.codec.Decode(msg.Payload)
	score := r.q.Score(msg.Size(), payload)

	assert.NoError(r.Suite.T(), payloadErr)
	// All of our mocked messages should have a score of 6
	assert.Equal(r.Suite.T(), 7, score)
}

// TestSingleMessageAdd adds a single message to the queue and verifies its existence
func (r *RcvQueueTestSuite) TestSingleMessageAdd() {
	msg := createMessage(0)

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	ctrl := gomock.NewController(r.Suite.T())
	defer ctrl.Finish()
	engines := make(map[uint8]network.Engine)
	engine := mocks.NewMockEngine(ctrl)
	payload, _ := r.q.codec.Decode(msg.Payload)
	engine.EXPECT().Process(senderID, payload).Return(nil).Times(1)
	engines[1] = engine
	r.q.SetEngines(&engines)

	added := r.q.Add(senderID, msg)

	// We should receive a true for added, since we have never seen that message before.
	assert.True(r.Suite.T(), added)
	// Cache should have one entry in it, since we're adding one message.
	assert.Equal(r.Suite.T(), 1, r.q.cache.Len())
	// Sleep to allow the message to be processed.
	time.Sleep(time.Second)
	// Queue should now be empty.
	assert.Equal(r.Suite.T(), 0, r.q.queue.Len())
	// Priority stack should be empty as well on our priority level.
	assert.Equal(r.Suite.T(), 0, r.q.stacks[5].Len())
}

// TestDuplicateSyncMessageAdd adds a single message twice to the queue and verifies its rejection in a synchronous manner
func (r *RcvQueueTestSuite) TestDuplicateSyncMessageAdd() {
	msg := createMessage(0)

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	ctrl := gomock.NewController(r.Suite.T())
	defer ctrl.Finish()
	engines := make(map[uint8]network.Engine)
	engine := mocks.NewMockEngine(ctrl)
	payload, _ := r.q.codec.Decode(msg.Payload)
	engine.EXPECT().Process(senderID, payload).Return(nil).Times(1)
	engines[1] = engine
	r.q.SetEngines(&engines)

	r.q.Add(senderID, msg)
	added := r.q.Add(senderID, msg)

	// We should receive a rejection since it is a duplicate message.
	assert.False(r.Suite.T(), added)
	time.Sleep(time.Second)
}

// TestDuplicateAsyncMessageAdd adds a single message twice to the queue and verifies its rejection in a asynchronous manner
func (r *RcvQueueTestSuite) TestDuplicateAsyncMessageAdd() {
	msg := createMessage(0)

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	ctrl := gomock.NewController(r.Suite.T())
	defer ctrl.Finish()
	engines := make(map[uint8]network.Engine)
	engine := mocks.NewMockEngine(ctrl)
	payload, _ := r.q.codec.Decode(msg.Payload)
	engine.EXPECT().Process(senderID, payload).Return(nil).Times(1)
	engines[1] = engine
	r.q.SetEngines(&engines)

	go r.q.Add(senderID, msg)
	go r.q.Add(senderID, msg)

	time.Sleep(time.Second)
	// Cache should have one entry in it, since we're adding the same message twice.
	assert.Equal(r.Suite.T(), 1, r.q.cache.Len())
	// Queue should now be empty.
	assert.Equal(r.Suite.T(), 0, r.q.queue.Len())
	// Priority stack should be empty as well on our priority level.
	assert.Equal(r.Suite.T(), 0, r.q.stacks[5].Len())
}

// TestOneHundredMessagesAdd tries to add 100 messages to the queue and confirms results.
func (r *RcvQueueTestSuite) TestOneHundredMessagesAdd() {
	msgs := make(map[int]*message.Message)
	for i := 99; i > -1; i-- {
		msgs[i] = createMessage(i * 2)
	}

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	ctrl := gomock.NewController(r.Suite.T())
	defer ctrl.Finish()
	engines := make(map[uint8]network.Engine)
	engine := mocks.NewMockEngine(ctrl)
	payload, _ := r.q.codec.Decode(msgs[0].Payload)
	engine.EXPECT().Process(senderID, payload).Return(nil).Times(100)
	engines[1] = engine
	r.q.SetEngines(&engines)

	for i := 99; i > -1; i-- {
		r.q.Add(senderID, msgs[i])
	}

	time.Sleep(time.Second * 2)
	// Cache should have 100 entries in it, since we're adding 100 messages.
	assert.Equal(r.Suite.T(), 100, r.q.cache.Len())
	// Queue should now be empty.
	assert.Equal(r.Suite.T(), 0, r.q.queue.Len())
	// Priority stack should be empty as well on our priority level.
	assert.Equal(r.Suite.T(), 0, r.q.stacks[5].Len())
}

// TestOneHundredMessagesWithPauseAdd tries to add 100 messages split into two groups, with a pause, to the queue and confirms results.
func (r *RcvQueueTestSuite) TestOneHundredMessagesWithPauseAdd() {
	msgs := make(map[int]*message.Message)
	for i := 99; i > -1; i-- {
		msgs[i] = createMessage(i * 2)
	}

	var sender [32]byte
	sender[0] = byte(2 + 1)
	senderID := flow.Identifier(sender)

	ctrl := gomock.NewController(r.Suite.T())
	defer ctrl.Finish()
	engines := make(map[uint8]network.Engine)
	engine := mocks.NewMockEngine(ctrl)
	payload, _ := r.q.codec.Decode(msgs[0].Payload)
	engine.EXPECT().Process(senderID, payload).Return(nil).Times(100)
	engines[1] = engine
	r.q.SetEngines(&engines)

	for i := 49; i > -1; i-- {
		r.q.Add(senderID, msgs[i])
	}

	time.Sleep(time.Second * 2)

	for i := 99; i > 49; i-- {
		r.q.Add(senderID, msgs[i])
	}

	time.Sleep(time.Second * 2)
	// Cache should have 100 entries in it, since we're adding 100 messages.
	assert.Equal(r.Suite.T(), 100, r.q.cache.Len())
	// Queue should now be empty.
	assert.Equal(r.Suite.T(), 0, r.q.queue.Len())
	// Priority stack should be empty as well on our priority level.
	assert.Equal(r.Suite.T(), 0, r.q.stacks[5].Len())
}
