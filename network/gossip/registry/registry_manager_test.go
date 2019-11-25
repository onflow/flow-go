package registry

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAddMessageType tests adding new msgTypes to the registry
func TestAddMessageType(t *testing.T) {
	assert := assert.New(t)
	r := NewRegistryManager(nil)
	defaultFunction := func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("Response"), nil
	}
	var (
		msgType1 MessageType
		msgType2 MessageType = 1
		msgType3 MessageType = 1
	)
	// In this test table the entry of test cases matters.
	tt := []struct {
		msgType MessageType
		err     error
	}{
		{
			msgType: msgType1,
			err:     nil,
		},
		{
			msgType: msgType2, //empty string msgType name
			err:     nil,
		},
		{ //duplicate msgType
			msgType: msgType3,
			err:     fmt.Errorf("non-nil"),
		},
	}

	for _, tc := range tt {
		err := r.AddMessageType(tc.msgType, defaultFunction, false)
		if tc.err != nil {
			assert.NotNil(err)
		}
		if err != nil && tc.err == nil {
			assert.Nil(err)
		}
	}
	assert.Equal(2, len(r.msgTypes))

	for i := 0; i < len(r.msgTypes); i++ {
		if _, ok := r.msgTypes[MessageType(i)]; !ok {
			t.Errorf("Error in message indexing. Expected function at index %v, Got none", i)
		}
	}

}

// TestInvokeMessageType tests invoking added msgTypes to the registry
func TestInvokeMessageType(t *testing.T) {
	assert := assert.New(t)
	r := NewRegistryManager(nil)
	var (
		msgType1 MessageType
		msgType2 MessageType = 1
		msgType3 MessageType = 2
		msgType4 MessageType = 17
	)

	_ = r.AddMessageType(msgType1, func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return []byte("Response"), nil
	}, false)
	_ = r.AddMessageType(msgType2, func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return payloadBytes, nil
	}, false)
	_ = r.AddMessageType(msgType3, func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return nil, fmt.Errorf("non nil")
	}, false)

	tt := []struct {
		msgType MessageType
		input   []byte
		resp    *invokeResponse
		err     error
	}{
		{
			msgType: msgType1,
			input:   []byte("tst"),
			resp:    &invokeResponse{Resp: []byte("Response"), Err: nil},
			err:     nil,
		},
		{
			msgType: msgType2,
			input:   []byte("Return this"),
			resp:    &invokeResponse{Resp: []byte("Return this"), Err: nil},
			err:     nil,
		},
		{ //msgType that returns an error
			msgType: msgType3,
			input:   []byte("tst"),
			resp:    &invokeResponse{Resp: nil, Err: fmt.Errorf("not nil")}, //We're only looking at the error for this test
			err:     nil,
		},
		{ //non-existing msgType
			msgType: msgType4,
			input:   []byte("tst"),
			resp:    nil,
			err:     fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {

		resp, err := r.Invoke(context.Background(), tc.msgType, tc.input)

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
