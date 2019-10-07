package gnode

import (
	"context"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

func TestAsyncQueue(t *testing.T) {
	a := NewNode(nil)
	go a.sweeper()

	//To test the error returned when a context provided is expired
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
		_, gotErr := a.AsyncQueue(tc.ctx, nil)
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
	a := NewNode(nil)
	//to handle the queue
	go a.sweeper()

	// registering a function
	err := a.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	genMsg := func(payload []byte, recipients []string, msgType string) *shared.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	//To test the error returned when a context provided is expired
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
		_, gotErr := a.SyncQueue(tc.ctx, tc.msg)

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
	a := NewNode(nil)

	//add a function for testing
	err := a.RegisterFunc("exists", func(ctx context.Context, Payload []byte) ([]byte, error) {
		return Payload, nil
	})

	if err != nil {
		t.Errorf("RegisterFunc: Expected nil error, Got: %v", err)
	}

	go a.sweeper()

	genMsg := func(payload []byte, recipients []string, msgType string) *shared.GossipMessage {
		msg, _ := generateGossipMessage(payload, recipients, msgType)
		return msg
	}

	tt := []struct {
		e   *entry
		err error
	}{
		{
			//nil entry
			e:   nil,
			err: fmt.Errorf("non nil"),
		},
		{ //entry with existing function
			e:   &entry{ctx: context.Background(), msg: genMsg([]byte("msg"), nil, "exists")},
			err: nil,
		},
		{ //entry with non-existing function
			e:   &entry{ctx: context.Background(), msg: genMsg([]byte("msg"), nil, "doesntexist")},
			err: fmt.Errorf("non nil"),
		},
		{ //entry with nil message
			e:   &entry{ctx: context.Background()},
			err: fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		gotErr := a.messageHandler(tc.e)
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
