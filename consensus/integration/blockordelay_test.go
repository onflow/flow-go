package integration_test

import (
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

// This file includes functions to simulate network conditions.
// The network conditions are simulated by defining whether a message sent to a receiver should be
// blocked or delayed.

type BlockOrDelayFunc func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration)

// block nothing
func blockNothing(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
	return false, 0
}

// block all messages sent by or received by a list of black listed nodes
func blockNodes(blackList ...*Node) BlockOrDelayFunc {
	blackDict := make(map[flow.Identifier]*Node, len(blackList))
	for _, n := range blackList {
		blackDict[n.id.ID()] = n
	}
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block, notBlock := true, false
		if _, ok := blackDict[sender.id.ID()]; ok {
			return block, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			return block, 0
		}
		return notBlock, 0
	}
}

// block all messages sent by or received by a list of black listed nodes for the first N messages
func blockNodesForFirstNMessages(n int, blackList ...*Node) BlockOrDelayFunc {
	blackDict := make(map[flow.Identifier]*Node, len(blackList))
	for _, n := range blackList {
		blackDict[n.id.ID()] = n
	}

	sent, received := 0, 0

	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block, notBlock := true, false

		switch m := event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		// case *messages.SyncRequest:
		// case *messages.SyncResponse:
		// case *messages.RangeRequest:
		// case *messages.BatchRequest:
		case *messages.BlockResponse:
			log := receiver.log.With().Int("blocks", len(m.Blocks)).Uint64("first", m.Blocks[0].Header.View).
				Uint64("last", m.Blocks[len(m.Blocks)-1].Header.View).Logger()
			log.Info().Msg("receives BlockResponse")
		default:
			return notBlock, 0
		}

		if _, ok := blackDict[sender.id.ID()]; ok {
			if sent >= n {
				return notBlock, 0
			}
			sent++
			return block, 0
		}
		if _, ok := blackDict[receiver.id.ID()]; ok {
			if received >= n {
				return notBlock, 0
			}
			received++
			return block, 0
		}
		return false, 0
	}
}

func blockReceiverMessagesByPercentage(percent int) BlockOrDelayFunc {
	rand.Seed(time.Now().UnixNano())
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block := rand.Intn(100) <= percent
		return block, 0
	}
}

func delayReceiverMessagesByRange(low time.Duration, high time.Duration) BlockOrDelayFunc {
	rand.Seed(time.Now().UnixNano())
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		rng := high - low
		delay := int64(low) + rand.Int63n(int64(rng))
		return false, time.Duration(delay)
	}
}
