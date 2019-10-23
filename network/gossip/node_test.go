package gossip

import (
	"context"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/network/gossip/order"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

var (
	defaultAddress = "127.0.0.1:50000"
)

func TestAsyncQueue(t *testing.T) {
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
		_, gotErr := gn.AsyncQueue(tc.ctx, &messages.GossipMessage{})
		if tc.err == nil && gotErr == nil {
			continue
		}

		if tc.err == nil && gotErr != nil {
			t.Errorf("AsyncQueue: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr == nil {
			t.Errorf("AsyncQueue: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr != nil {
			continue
		}
	}
}

func TestSyncQueue(t *testing.T) {
	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)
	//to handle the queue
	go gn.sweeper()

	// registering gn function
	err := gn.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	genMsg := func(payload []byte, recipients []string, msgType uint64) *messages.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	//To test the error returned when gn context provided is expired
	expiredContext, cancel := context.WithCancel(context.Background())
	// cancelling the context
	cancel()

	tt := []struct {
		ctx context.Context
		msg *messages.GossipMessage
		err error
	}{
		{ //Working example
			ctx: context.Background(),
			msg: genMsg([]byte("msg"), nil, 3), //3 is the index of the first registered function (functions 0-2 are reserved)
			err: nil,
		},
		{ // Expired context
			ctx: expiredContext,
			msg: genMsg([]byte("msg"), nil, 3),
			err: fmt.Errorf("non nil"),
		},
		{ //Invalid function
			ctx: context.Background(),
			msg: genMsg([]byte("msg"), nil, 5), //5 is the index of an inexistant function
			err: fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		_, gotErr := gn.SyncQueue(tc.ctx, tc.msg)

		if tc.err == nil && gotErr == nil {
			continue
		}

		if tc.err == nil && gotErr != nil {
			t.Errorf("SyncQueue: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr == nil {
			t.Errorf("SyncQueue: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr != nil {
			continue
		}
	}
}

func TestMessageHandler(t *testing.T) {
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

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	go gn.sweeper()

	genMsg := func(payload []byte, recipients []string, msgType string) *messages.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, getMsgID(msgType))
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

		if tc.err == nil && gotErr != nil {
			t.Errorf("MessageHandler: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr == nil {
			t.Errorf("MessageHandler: Expected %v, Got: %v", tc.err, gotErr)
		}

		if tc.err != nil && gotErr != nil {
			continue
		}
	}
}

func TestTryStore(t *testing.T) {

	config := NewNodeConfig(nil, defaultAddress, []string{}, 0, 10)
	gn := NewNode(config)

	//generating two messages with different script
	msg1, _ := generateGossipMessage([]byte("hello"), []string{}, 3)
	msg2, _ := generateGossipMessage([]byte("hi"), []string{}, 4)

	// pretending that the node received msg2
	h2, _ := computeHash(msg2)
	gn.hashCache.receive(string(h2))

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
		if rep != tc.expectedBool {
			t.Errorf("tryStore: Expected: %v, Got: %v", tc.expectedBool, rep)
		}
	}
}

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

	if size := len(n.fanoutSet); size != 2 {
		t.Errorf("expected a new fanout set of size 2, received fanout set of size: %v", size)
	}
}
