package integration_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

type BlockOrDelayFunc func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration)

func blockNothing(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
	return false, 0
}

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
			log := receiver.log.With().Int("blocks", len(m.Blocks)).Uint64("first", m.Blocks[0].View).
				Uint64("last", m.Blocks[len(m.Blocks)-1].View).Logger()
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

func blockProposals() BlockOrDelayFunc {
	return func(channelID uint8, event interface{}, sender, receiver *Node) (bool, time.Duration) {
		switch event.(type) {
		case *messages.BlockProposal:
		case *messages.BlockVote:
		case *messages.SyncRequest:
		case *messages.SyncResponse:
		case *messages.RangeRequest:
		case *messages.BatchRequest:
		case *messages.BlockResponse:
		default:
			panic(fmt.Sprintf("wrong message from channel %v, type: %v", channelID, reflect.TypeOf(event)))
		}
		return false, 0
	}
}
