package gnode

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip/v1/order"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var (
	defaultLogger  = zerolog.New(ioutil.Discard)
	defaultAddress = "127.0.0.1:50000"
)

// TestAsyncQueue tests the performance of AsyncQueue function
func TestAsyncQueue(t *testing.T) {
	assert := assert.New(t)
	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)
	go gn.sweeper()


	//To test the error returned when gn context provided is expired
	expiredContext, cancel := context.WithCancel(context.Background())
	//expiring the context
	cancel()

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
		_, gotErr := gn.AsyncQueue(tc.ctx, &shared.GossipMessage{})
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

// TestSyncQueue tests the performance of SyncQueue function
func TestSyncQueue(t *testing.T) {
	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)

	assert := assert.New(t)


	//to handle the queue
	go gn.sweeper()

	// registering gn function
	err := gn.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})
	assert.Nil(err)


	genMsg := func(payload []byte, recipients []string, msgType uint64) *shared.GossipMessage {
		msg := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	//To test the error returned when gn context provided is expired
	expiredContext, cancel := context.WithCancel(context.Background())
	// // cancelling the context
	cancel()

	tt := []struct {
		ctx context.Context
		msg *shared.GossipMessage
		err error
	}{
		{ //Working example
			ctx: context.Background(),
			msg: genMsg([]byte("msg1"), nil, 3), //3 is the index of the first registered function (functions 0-2 are reserved)
			err: nil,
		},
		{ // Expired context
			ctx: expiredContext,
			msg: genMsg([]byte("msg2"), nil, 3),
			err: fmt.Errorf("non nil"),
		},
		{ //Invalid function
			ctx: context.Background(),
			msg: genMsg([]byte("msg3"), nil, 5), //5 is the index of an non-existing function
			err: fmt.Errorf("non nil"),
		},

	}

	for _, tc := range tt {
		_, gotErr := gn.SyncQueue(tc.ctx, tc.msg)

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
	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)
	getMsgID := func(msgType string) uint64 {
		id, err := gn.regMngr.MsgTypeToID(msgType)
		if err != nil {
			return 1000
		}
		return id
	}



	//add gn function for testing
	err := gn.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})
	assert.Nil(err)

	go gn.sweeper()

	genMsg := func(payload []byte, recipients []string, msgType string) *shared.GossipMessage {
		msg := generateGossipMessage(payload, recipients, getMsgID(msgType))
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
			e:   order.NewSync(context.Background(), genMsg([]byte("msg"), nil, "exists")),
			err: nil,
		},
		{ //entry with non-existing function
			e:   order.NewSync(context.Background(), genMsg([]byte("msg"), nil, "doesntexist")),
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

	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)



	//generating two messages with different script
	msg1 := generateGossipMessage([]byte("hello"), []string{}, 3)
	msg2 := generateGossipMessage([]byte("hi"), []string{}, 4)

	// pretending that the node received msg2
	h2, _ := computeHash(msg2)
	gn.hashCache.receive(string(h2))

	tt := []struct {
		msg          *shared.GossipMessage
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
	n.pickGossipPartners()
	assert.Equal(t, 2, len(n.fanoutSet))
}
