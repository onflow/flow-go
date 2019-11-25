package gossip

import (
	"context"
	"fmt"
	"github.com/dapperlabs/flow-go/network/gossip/util"
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/network/gossip/order"
	"github.com/dapperlabs/flow-go/network/gossip/registry"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

var (
	defaultAddress = "127.0.0.1:50000"
)

// TestQueueService tests the performance of QueueService function
func TestQueueService(t *testing.T) {
	assert := assert.New(t)
	gn := NewNode(WithLogger(zerolog.New(ioutil.Discard)), WithAddress(defaultAddress))
	go gn.sweeper()

	//To test the error returned when gn context provided is expired
	expiredContext, cancel := context.WithCancel(context.Background())
	//expiring the context
	cancel()

	msg, _ := generateGossipMessage([]byte(""), []string{}, 17)

	tt := []struct {
		ctx context.Context
		err error
	}{
		{ //Working test
			ctx: context.Background(),
			err: nil,
		},
		{ //Cancelled context
			ctx: expiredContext,
			err: fmt.Errorf("non nil"),
		},
	}
	for _, tc := range tt {
		_, gotErr := gn.QueueService(tc.ctx, msg)
		if tc.err == nil && gotErr == nil {
			continue
		}

		if tc.err == nil {
			assert.Nil(gotErr)
		}

		if tc.err != nil {
			assert.NotNil(gotErr)
		}

	}
}

// TestMessageHandler tests the performance of messageHandler function
func TestMessageHandler(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	gn := NewNode(WithLogger(zerolog.New(ioutil.Discard)), WithAddress(defaultAddress))

	var (
		msgType1 registry.MessageType = 4
		msgType2 registry.MessageType = 17
	)

	//add gn function for testing
	err := gn.RegisterFunc(msgType1, func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})
	assert.Nil(err)

	go gn.sweeper()

	genMsg := func(payload []byte, recipients []string, msgType registry.MessageType) *messages.GossipMessage {
		msg, err := generateGossipMessage(payload, recipients, uint64(msgType))
		require.Nil(err, "non-nil error")
		return msg
	}

	tt := []struct {
		e   *order.Order
		err error
	}{
		{ //nil entry
			e:   nil,
			err: fmt.Errorf("non nil"),
		},
		{ //entry with existing function
			e:   order.NewSync(context.Background(), genMsg([]byte("msg"), nil, msgType1)),
			err: nil,
		},
		{ //entry with non-existing function
			e:   order.NewSync(context.Background(), genMsg([]byte("msg"), nil, msgType2)),
			err: fmt.Errorf("non nil"),
		},
		{ //entry with nil message
			e:   order.NewSync(context.Background(), nil),
			err: fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		gotErr := gn.messageHandler(tc.e)
		if tc.err == nil && gotErr == nil {
			continue
		}
		if tc.err == nil {
			assert.Nil(gotErr)
		}
		if tc.err != nil {
			assert.NotNil(gotErr)
		}
	}
}

// TestTryStore tests the performance of tryStore function
func TestTryStore(t *testing.T) {
	assert := assert.New(t)

	gn := NewNode(WithLogger(zerolog.New(ioutil.Discard)), WithAddress(defaultAddress))

	//generating two messages with different script
	msg1, _ := generateGossipMessage([]byte("hello"), []string{}, 3)
	msg2, _ := generateGossipMessage([]byte("hi"), []string{}, 4)

	// pretending that the node received msg2
	h2, _ := util.ComputeHash(msg2)
	gn.hashCache.Receive(string(h2))

	tt := []struct {
		msg          *messages.GossipMessage
		expectedBool bool
	}{
		{ // msg1 is not stored in the store
			msg:          msg1,
			expectedBool: false,
		},
		{ // msg1 is in the store
			msg:          msg2,
			expectedBool: true,
		},
	}

	// test if storage reports the correct state of the message
	for _, tc := range tt {
		rep, _ := gn.tryStore(tc.msg)
		assert.Equal(tc.expectedBool, rep)

	}
}

// TestPickRandom tests the performance of pickGossipPartners function
func TestPickRandom(t *testing.T) {
	n := &Node{}
	n.peers = []string{
		"Wyatt",
		"Jayden",
		"John",
		"Owen",
		"Dylan",
		"Luke",
		"Gabriel",
		"Anthony",
		"Isaac",
		"Grayson",
		"Jack",
		"Julian",
		"Levi",
		"Christopher",
		"Joshua",
		"Andrew",
		"Lincoln",
	}

	n.staticFanoutNum = 2
	_ = n.pickGossipPartners()

	require.Len(t, n.fanoutSet, 2)
}
