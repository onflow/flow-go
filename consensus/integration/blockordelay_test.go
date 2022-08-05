package integration_test

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
)

// This file includes functions to simulate network conditions.
// The network conditions are simulated by defining whether a message sent to a receiver should be
// blocked or delayed.

// blockNodesFirstMessages blocks n incoming messages to given nodes
func blockNodesFirstMessages(n uint64, denyList ...*Node) BlockOrDelayFunc {
	blackList := make(map[flow.Identifier]uint64, len(denyList))
	for _, node := range denyList {
		blackList[node.id.ID()] = n
	}
	return func(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		// filter only consensus messages
		switch event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		case *messages.BlockResponse:
		case *messages.TimeoutObject:
		default:
			return false, 0
		}
		count, ok := blackList[receiver.id.ID()]
		if ok && count > 0 {
			blackList[receiver.id.ID()] = count - 1
			return true, 0
		}
		return false, 0
	}
}

func blockReceiverMessagesByPercentage(percent int) BlockOrDelayFunc {
	rand.Seed(time.Now().UnixNano())
	return func(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block := rand.Intn(100) <= percent
		return block, 0
	}
}

func delayReceiverMessagesByRange(low time.Duration, high time.Duration) BlockOrDelayFunc {
	rand.Seed(time.Now().UnixNano())
	return func(channel network.Channel, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		rng := high - low
		delay := int64(low) + rand.Int63n(int64(rng))
		return false, time.Duration(delay)
	}
}
