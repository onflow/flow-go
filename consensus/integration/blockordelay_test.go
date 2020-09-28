// +build timesensitivetest

package integration_test

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// This file includes functions to simulate network conditions.
// The network conditions are simulated by defining whether a message sent to a receiver should be
// blocked or delayed.

// block all messages sent by or received by a list of denied nodes for the first N messages
func blockNodesForFirstNMessages(n int, denyList ...*Node) BlockOrDelayFunc {
	denyDict := make(map[flow.Identifier]*Node, len(denyList))
	for _, n := range denyList {
		denyDict[n.id.ID()] = n
	}

	sent, received := 0, 0

	return func(channelID string, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block, notBlock := true, false

		switch m := event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		case *messages.BlockResponse:
			log := receiver.log.With().Int("blocks", len(m.Blocks)).Uint64("first", m.Blocks[0].Header.View).
				Uint64("last", m.Blocks[len(m.Blocks)-1].Header.View).Logger()
			log.Info().Msg("receives BlockResponse")
		case *messages.SyncRequest:
		case *messages.SyncResponse:
		case *messages.RangeRequest:
		case *messages.BatchRequest:
		default:
			return notBlock, 0
		}

		if _, ok := denyDict[sender.id.ID()]; ok {
			if sent >= n {
				return notBlock, 0
			}
			sent++
			return block, 0
		}
		if _, ok := denyDict[receiver.id.ID()]; ok {
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
	return func(channelID string, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		block := rand.Intn(100) <= percent
		return block, 0
	}
}

func delayReceiverMessagesByRange(low time.Duration, high time.Duration) BlockOrDelayFunc {
	rand.Seed(time.Now().UnixNano())
	return func(channelID string, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		rng := high - low
		delay := int64(low) + rand.Int63n(int64(rng))
		return false, time.Duration(delay)
	}
}
