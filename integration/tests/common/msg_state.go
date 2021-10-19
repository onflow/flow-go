package common

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

const msgStateTimeout = 20 * time.Second

type MsgState struct {
	msgs sync.Map
}

func (ms *MsgState) Add(sender flow.Identifier, msg interface{}) {
	var list []interface{}
	value, ok := ms.msgs.Load(sender)

	if !ok {
		list = make([]interface{}, 0)
	} else {
		list = value.([]interface{})
	}

	list = append(list, msg)
	ms.msgs.Store(sender, list)
}

// From returns a slice with all the msgs received from the given node and a boolean whether any messages existed
func (ms *MsgState) From(node flow.Identifier) ([]interface{}, bool) {
	msgs, ok := ms.msgs.Load(node)
	if !ok {
		return nil, ok
	}
	return msgs.([]interface{}), ok
}

// LenFrom returns the number of msgs received from the given node
func (ms *MsgState) LenFrom(node flow.Identifier) int {
	msgs, ok := ms.msgs.Load(node)
	if !ok {
		return 0
	}

	return len(msgs.([]interface{}))
}

// WaitForMsgFrom waits for a msg satisfying the predicate from the given node and returns it
func (ms *MsgState) WaitForMsgFrom(t *testing.T, predicate func(msg interface{}) bool, node flow.Identifier, msg string) interface{} {
	var m interface{}
	i := 0
	require.Eventually(t, func() bool {
		if value, ok := ms.msgs.Load(node); ok {
			list := value.([]interface{})
			for ; i < len(list); i++ {
				if predicate(list[i]) {
					m = list[i]
					return true
				}
			}
		}

		return false
	}, msgStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive msg %s from %x within %v seconds \n", msg, node,
			msgStateTimeout))
	return m
}

func MsgIsChunkDataRequest(msg interface{}) bool {
	_, ok := msg.(*messages.ChunkDataRequest)
	return ok
}

func MsgIsChunkDataPackResponse(msg interface{}) bool {
	_, ok := msg.(*messages.ChunkDataResponse)
	return ok
}

func MsgIsResultApproval(msg interface{}) bool {
	_, ok := msg.(*flow.ResultApproval)
	return ok
}

func MsgIsExecutionStateDelta(msg interface{}) bool {
	_, ok := msg.(*messages.ExecutionStateDelta)
	return ok
}

func MsgIsExecutionStateDeltaWithChanges(msg interface{}) bool {
	delta, ok := msg.(*messages.ExecutionStateDelta)
	if !ok {
		return false
	}

	return *delta.StartState != delta.EndState
}
