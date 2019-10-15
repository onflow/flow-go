package gnode

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// TestAddMessageType tests adding new msgTypes to the registry
func TestAddMessageType(t *testing.T) {
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
		if err != nil && tc.err != nil {
			continue
		}
		if err == nil && tc.err == nil {
			continue
		}
		if err == nil && tc.err != nil {
			t.Errorf("AddMessageType: Expected an error, Got no error")
		}
		if err != nil && tc.err == nil {
			t.Errorf("AddMessageType: Expected no error, Got an error")
		}
	}

	if (len(r.msgTypes)) != 3 {
		t.Errorf("AddMessageType: Registry size mismatch, Expected 3, Got %v", len(r.msgTypes))
	}

}

//TestInvokeMessageType tests invoking added msgTypes to the registry
func TestInvokeMessageType(t *testing.T) {
	availableMessageTypes := make(map[string]HandleFunc)

	availableMessageTypes["exists"] = func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return []byte("Response"), nil
	}
	availableMessageTypes["returnsInp"] = func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return payloadBytes, nil
	}
	availableMessageTypes["returnsError"] = func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
		return nil, fmt.Errorf("non nil")
	}

	//MultiRegistry that contains the msgTypes defined above
	mr := &MultiRegistry{msgTypes: availableMessageTypes}

	//Registry Manager that wraps mr in entry to test the invoke functions
	r := newRegistryManager(mr)

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
		resp, err := r.Invoke(context.Background(), tc.msgType, tc.input)

		if err != nil && tc.err != nil {
			continue
		}
		if err == nil && tc.err != nil {
			t.Errorf("AddMessageType: Expected an error, Got no error")
		}
		if err != nil && tc.err == nil {
			t.Errorf("AddMessageType: Expected no error, Got an error")
		}
		if err == nil && tc.err == nil {
			if tc.resp.Err != nil && resp.Err == nil {
				t.Errorf("Invoke: Expected an error, Got no error")
			}

			if tc.resp.Err == nil && resp.Err != nil {
				t.Errorf("Invoke: Expected no error, Got an error")
			}

			if tc.resp.Err != nil && resp.Err != nil {
				continue
			}

			if !reflect.DeepEqual(resp.Resp, tc.resp.Resp) {
				t.Errorf("Invocation: Output Expected: %v, Got: %v", string(tc.resp.Resp), string(resp.Resp))
			}
		}
	}

}

//Test combining multiple multi registries into a single registry
func TestNewMultiRegistry(t *testing.T) {

	availableMessageType1 := map[string]HandleFunc{
		"myfunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
		"myduplicatefunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
	}
	mr1 := &MultiRegistry{msgTypes: availableMessageType1}

	availableMessageType2 := map[string]HandleFunc{
		"myotherfunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
		"myduplicatefunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
	}
	mr2 := &MultiRegistry{msgTypes: availableMessageType2}

	availableMessageType3 := map[string]HandleFunc{
		"myuniquefunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
		"anotheruniquefunction": func(ctx context.Context, payloadBytes []byte) ([]byte, error) {
			return []byte("Response"), nil
		},
	}
	mr3 := &MultiRegistry{msgTypes: availableMessageType3}

	//with duplicates
	combinedRegistries := NewMultiRegistry(mr1, mr2, mr3)

	if len(combinedRegistries.MessageTypes()) != 5 {
		t.Errorf("NewMultiRegistry: Size Expected: 5, Got: %v", len(combinedRegistries.MessageTypes()))
	}

	//without duplicates
	combinedRegistries = NewMultiRegistry(mr1, mr3)
	if len(combinedRegistries.MessageTypes()) != 4 {
		t.Errorf("NewMultiRegistry: Size Expected: 4, Got: %v", len(combinedRegistries.MessageTypes()))
	}
}
