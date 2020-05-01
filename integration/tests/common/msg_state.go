package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

const msgStateTimeout = 10 * time.Second

type MsgState struct {
	msgs map[flow.Identifier][]interface{}
}

func (ms *MsgState) Add(sender flow.Identifier, msg interface{}) {
	if ms.msgs == nil {
		ms.msgs = make(map[flow.Identifier][]interface{})
	}

	ms.msgs[sender] = append(ms.msgs[sender], msg)
}

// From returns a slice with all the msgs received from the given node and a boolean whether any messages existed
func (ms *MsgState) From(node flow.Identifier) ([]interface{}, bool) {
	msgs, ok := ms.msgs[node]
	return msgs, ok
}

// LenFrom returns the number of msgs received from the given node
func (ms *MsgState) LenFrom(node flow.Identifier) int {
	msgs, ok := ms.msgs[node]
	if !ok {
		return 0
	}
	return len(msgs)
}

// WaitForAtFrom waits for a msg satisfying the predicate from the given node and returns it
func (ms *MsgState) WaitForAtFrom(t *testing.T, predicate func(msg interface{}) bool, node flow.Identifier) interface{} {
	var m interface{}
	i := 0
	require.Eventually(t, func() bool {
		msgs, ok := ms.msgs[node]

		if !ok {
			return false
		}

		for ; i < len(msgs); i++ {
			if predicate(msgs[i]) {
				m = msgs[i]
				return true
			}
		}

		return false
	}, msgStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive msg satisfying predicate from %x within %v seconds", node,
			msgStateTimeout))
	return m
}

func MsgIsChunkDataPackRequest(msg interface{}) bool {
	_, ok := msg.(*messages.ChunkDataPackRequest)
	return ok
}
