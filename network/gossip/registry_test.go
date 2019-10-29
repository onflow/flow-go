package gossip

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

//// TestAddMessageType tests adding new msgTypes to the registry
func TestAddMessageType(t *testing.T) {
	assert := assert.New(t)
	r := newRegistryManager(nil)

	defaultFunction := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("Response"), nil
	}

	//In this test table the entry of test cases matters.
	tt := []struct {
		msgType string
		err     error
	}{
		{
			msgType: "exists",
			err:     nil,
		},
		{
			msgType: "alsoExists",
			err:     nil,
		},
		{
			msgType: "", //empty string msgType name
			err:     fmt.Errorf("non-nil"),
		},
		{ //duplicate msgType
			msgType: "exists",
			err:     fmt.Errorf("non-nil"),
		},
		{
			msgType: "last",
			err:     nil,
		},
	}

	for _, tc := range tt {
		err := r.AddMessageType(tc.msgType, defaultFunction)
		if tc.err != nil {
			assert.NotNil(err)
		}
		if err != nil && tc.err == nil {
			assert.Nil(err)
		}
	}
	assert.Equal(3, len(r.msgTypes))

	for i := 0; i < len(r.msgTypes); i++ {
		if _, ok := r.msgTypes[uint64(i)]; !ok {
			t.Errorf("Error in message indexing. Expected function at index %v, Got none", i)
		}
	}

}

// TestInvokeMessageType tests invoking added msgTypes to the registry
func TestInvokeMessageType(t *testing.T) {
	assert := assert.New(t)
	r := newRegistryManager(nil)

	_ = r.AddMessageType("exists", func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return []byte("Response"), nil
	})
	_ = r.AddMessageType("returnsInp", func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return payloadBytes, nil
	})
	_ = r.AddMessageType("returnsError", func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return nil, fmt.Errorf("non nil")
	})

	tt := []struct {
		msgType string
		input   []byte
		resp    *invokeResponse
		err     error
	}{
		{
			msgType: "exists",
			input:   []byte("tst"),
			resp:    &invokeResponse{Resp: []byte("Response"), Err: nil},
			err:     nil,
		},
		{
			msgType: "returnsInp",
			input:   []byte("Return this"),
			resp:    &invokeResponse{Resp: []byte("Return this"), Err: nil},
			err:     nil,
		},
		{ //msgType that returns an error
			msgType: "returnsError",
			input:   []byte("tst"),
			resp:    &invokeResponse{Resp: nil, Err: fmt.Errorf("not nil")}, //We're only looking at the error for this test
			err:     nil,
		},
		{ //non-existing msgType
			msgType: "doesntexist",
			input:   []byte("tst"),
			resp:    nil,
			err:     fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		msgID, err := r.MsgTypeToID(tc.msgType)
		if tc.err != nil {
			assert.NotNil(err)
		}

		if tc.err == nil {
			assert.Nil(err)
		}

		if err != nil && tc.err != nil {
			continue
		}

		resp, err := r.Invoke(context.Background(), msgID, tc.input)

		if tc.err != nil {
			assert.NotNil(err)
		}
		if tc.err == nil {
			assert.Nil(err)
		}
		if err == nil && tc.err == nil {
			if tc.resp.Err != nil {
				assert.NotNil(resp.Err)
			}

			if tc.resp.Err == nil {
				assert.Nil(resp.Err)
			}

			if !reflect.DeepEqual(resp.Resp, tc.resp.Resp) {
				t.Errorf("Invocation: Output Expected: %v, Got: %v", string(tc.resp.Resp), string(resp.Resp))
			}
		}
	}

}
