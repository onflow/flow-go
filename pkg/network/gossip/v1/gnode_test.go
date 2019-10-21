package gnode

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip/v1/order"
	"github.com/rs/zerolog"
)

var defaultLogger zerolog.Logger = zerolog.New(ioutil.Discard)

func TestAsyncQueue(t *testing.T) {
	a := NewNode(nil, "", []string{}, 0, 10)
	go a.sweeper()


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
		_, gotErr := a.AsyncQueue(tc.ctx, &shared.GossipMessage{})
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
	gn := NewNode(nil, "", []string{}, 0, 10)
	//to handle the queue
	go gn.sweeper()

	// registering gn function
	err := gn.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	genMsg := func(payload []byte, recipients []string, msgType string) *shared.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	//To test the error returned when gn context provided is expired
	expiredContext, cancel := context.WithCancel(context.Background())
	// cancelling the context
	cancel()

	tt := []struct {
		ctx context.Context
		msg *shared.GossipMessage
		err error
	}{
		{ //Working example
			ctx: context.Background(),
			msg: genMsg([]byte("msg"), nil, "exists"),
			err: nil,
		},
		{ // Expired context
			ctx: expiredContext,
			msg: genMsg([]byte("msg"), nil, "exists"),
			err: fmt.Errorf("non nil"),
		},
		{ //Invalid function
			ctx: context.Background(),
			msg: genMsg([]byte("msg"), nil, "doesntExist"),
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
	gn := NewNode(nil, "", []string{}, 0, 10)

	//add gn function for testing
	err := gn.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	go gn.sweeper()

	genMsg := func(payload []byte, recipients []string, msgType string) *shared.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	tt := []struct {
		e   *order.Order
		err error
	}{
		{
			//nil entry
			e:   nil,
			err: fmt.Errorf("non nil"),
		},
		{ //entry with existing function
			e:   order.NewOrder(context.Background(), genMsg([]byte("msg"), nil, "exists"), true),
			err: nil,
		},
		{ //entry with non-existing function
			e:   order.NewOrder(context.Background(), genMsg([]byte("msg"), nil, "doesntexist"), true),
			err: fmt.Errorf("non nil"),
		},
		{ //entry with nil message
			e:   order.NewOrder(context.Background(), nil, true),
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

	node := NewNode(nil, "", []string{}, 0, 10)

	//generating two messages with different script
	msg1, _ := generateGossipMessage([]byte("hello"), []string{}, "")
	msg2, _ := generateGossipMessage([]byte("hi"), []string{}, "")


	// pretending that the node received msg2
	h2, _ := computeHash(msg2)
	node.hashCache.receive(string(h2))

	tt := []struct {
		msg          *shared.GossipMessage
		expectedBool bool
	}{
		{// msg1 is not stored in the store
			msg:          msg1,
			expectedBool: false,
		},
		{// msg1 is in the store
			msg:          msg2,
			expectedBool: true,
		},
	}

	// test if storage reports the correct state of the message
	for _, tc := range tt {
		rep, _ := node.tryStore(tc.msg)
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
	n.pickGossipPartners()

	if size := len(n.fanoutSet); size != 2 {
		t.Errorf("expected a new fanout set of size 2, received fanout set of size: %v", size)
	}
}
