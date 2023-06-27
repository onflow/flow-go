package integration_test

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
)

// This file includes functions to simulate network conditions.
// The network conditions are simulated by defining whether a message sent to a receiver should be
// blocked or delayed.

// blockNodesFirstMessages blocks the _first_ n incoming messages for each member in `denyList`
// (messages are counted individually for each Node).
func blockNodesFirstMessages(n uint64, denyList ...*Node) BlockOrDelayFunc {
	blackList := make(map[flow.Identifier]uint64, len(denyList))
	for _, node := range denyList {
		blackList[node.id.ID()] = n
	}
	lock := new(sync.Mutex)
	return func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		// filter only consensus messages
		switch event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		case *messages.BlockResponse:
		case *messages.TimeoutObject:
		default:
			return false, 0
		}
		lock.Lock()
		defer lock.Unlock()
		count, ok := blackList[receiver.id.ID()]
		if ok && count > 0 {
			blackList[receiver.id.ID()] = count - 1
			return true, 0
		}
		return false, 0
	}
}

// blockReceiverMessagesRandomly drops messages randomly with a probability of `dropProbability` ∈ [0,1]
func blockReceiverMessagesRandomly(dropProbability float32) BlockOrDelayFunc {
	lock := new(sync.Mutex)
	prng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		lock.Lock()
		block := prng.Float32() < dropProbability
		lock.Unlock()
		return block, 0
	}
}

// delayReceiverMessagesByRange delivers all messages, but with a randomly sampled
// delay in the interval [low, high). Panics when `low` < 0 or `low > `high`
func delayReceiverMessagesByRange(low time.Duration, high time.Duration) BlockOrDelayFunc {
	lock := new(sync.Mutex)
	prng := rand.New(rand.NewSource(time.Now().UnixNano()))
	delayRangeNs := int64(high - low)
	minDelayNs := int64(low)

	// fail early for non-sensical parameter settings
	if int64(low) < 0 {
		panic(fmt.Sprintf("minimal delay cannot be negative, but is %d ns", int64(low)))
	}
	if delayRangeNs < 0 {
		panic(fmt.Sprintf("upper bound on delay (%d ns) cannot be smaller than lower bound (%d ns)", int64(high), int64(low)))
	}

	// shortcut for low = high: always return low
	if delayRangeNs == 0 {
		return func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
			return false, low
		}
	}
	// general version
	return func(channel channels.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		lock.Lock()
		d := prng.Int63n(delayRangeNs)
		lock.Unlock()
		return false, time.Duration(minDelayNs + d)
	}
}
