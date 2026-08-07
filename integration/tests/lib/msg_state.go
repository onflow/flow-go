package lib

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

const msgStateTimeout = 20 * time.Second

type MsgState struct {
	msgs sync.Map
}

func NewMsgState() *MsgState {
	return &MsgState{}
}

func (ms *MsgState) Add(sender flow.Identifier, msg any) {
	var list []any
	value, ok := ms.msgs.Load(sender)

	if !ok {
		list = make([]any, 0)
	} else {
		list = value.([]any)
	}

	list = append(list, msg)
	ms.msgs.Store(sender, list)
}

// From returns a slice with all the msgs received from the given node and a boolean whether any messages existed
func (ms *MsgState) From(node flow.Identifier) ([]any, bool) {
	msgs, ok := ms.msgs.Load(node)
	if !ok {
		return nil, ok
	}
	return msgs.([]any), ok
}

// LenFrom returns the number of msgs received from the given node
func (ms *MsgState) LenFrom(node flow.Identifier) int {
	msgs, ok := ms.msgs.Load(node)
	if !ok {
		return 0
	}

	return len(msgs.([]any))
}

// WaitForMsgFrom waits for a msg satisfying the predicate from the given node and returns it
func (ms *MsgState) WaitForMsgFrom(t *testing.T, predicate func(msg any) bool, node flow.Identifier, msg string) any {
	var m any
	i := 0
	require.Eventually(t, func() bool {
		if value, ok := ms.msgs.Load(node); ok {
			list := value.([]any)
			for ; i < len(list); i++ {
				if predicate(list[i]) {
					m = list[i]
					return true
				}
			}
		}

		return false
	}, msgStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive msg %s from %x within %v seconds", msg, node,
			msgStateTimeout))
	return m
}

func MsgIsChunkDataPackResponse(msg any) bool {
	_, ok := msg.(*flow.ChunkDataResponse)
	return ok
}
