package gossip

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

//TestDatabase covers several storage and retrieval of data to and from database considering different scenarios
func TestDatabase(t *testing.T) {
	mmd := newMemMsgDatabase()

	initKeys := []string{
		"exists",
		"found",
	}

	//Filling out the test table with:
	//keys from initKeys slice
	//value for each key is a GossipMessage with payload as byte representation of the key, and
	//type as the key
	for _, key := range initKeys {
		err := mmd.Put(key, &messages.GossipMessage{})
		if err != nil {
			t.Errorf("error putting: %v. expected: nil error, got: non nil error", key)
		}
		assert.Nil(t, err)
	}

	tt := []struct {
		item string
		err  error
	}{
		{ //an existing item
			item: "exists",
			err:  nil,
		},
		{ //an existing item
			item: "found",
			err:  nil,
		},
		{ //a non-existing item
			item: "doesntexist",
			err:  fmt.Errorf("non nil"),
		},
		{ //a non-existing item
			item: "notfound",
			err:  fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		message, err := mmd.Get(tc.item)
		if err == tc.err {
			continue
		}
		if err != nil && tc.err == nil {
			t.Errorf("error in get. Expected: nil error, Got: non nil error")
		}

		if err == nil && tc.err != nil {
			t.Errorf("error in get. Expected: non nil error, Got: nil error")
		}
		if message != nil && string(message.Payload) != tc.item {
			t.Errorf("error in get over message type. Expected: %v, Got: %v", tc.item, message.MessageType)
		}
		if message != nil && string(message.Payload) != tc.item {
			t.Errorf("error in get over message type. Expected: %v, Got: %v", tc.item, string(message.Payload))
		}
	}
}
