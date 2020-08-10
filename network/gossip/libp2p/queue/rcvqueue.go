package queue

import (
	"fmt"
	"runtime"
	"math"

	"github.com/rs/zerolog"
	lru "github.com/hashicorp/golang-lru"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
)

// RcvQueue implements an LRU cache of the received eventIDs that delivered to their engines
type RcvQueue struct {
	logger  zerolog.Logger
	size		int
	workers int
	max     int
	codec		network.Codec
	cache   *lru.Cache // The LRU cache we use for de-duplication.
	queue		*lru.Cache // The LRU cache we use for overflow.
	stacks  [10]*lru.Cache // Ten LRU caches we use for priority organization.
	engines *map[uint8]network.Engine // The engines we need to execute the message.
}

// RcvQueueKey represents a key for the cache and queue
type RcvQueueKey struct {
	eventID   string
	channelID uint8
}

// RcvQueueValue represents a value for the queue
type RcvQueueValue struct {
	senderID	flow.Identifier
	message		*message.Message
}

// RcvStackValue represents a value for the queue
type RcvStackValue struct {
	senderID				flow.Identifier
	decodedMessage	interface{}
}

// NewRcvQueue creates and returns a new RcvQueue
func NewRcvQueue(log zerolog.Logger, size int) (*RcvQueue, error) {
	// Determine the maximum number of cores we can use for processing
	var max int
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		max = maxProcs
	} else {
		max = numCPU
	}

	// We will need the codec to decode messages to determine the message type
	codec := jsoncodec.NewCodec()

	// Create our de-duplication cache
	cache, err := lru.New(size * 10e3)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	// Create our overflow queue
	queue, err := lru.New(size)
	if err != nil {
		return nil, fmt.Errorf("could not initialize queue: %w", err)
	}

	// Create our priority queues
	var stacks [10]*lru.Cache
	for i := 0; i < 10; i++ {
		stacks[i], err = lru.New(size * 100)
		if err != nil {
			return nil, fmt.Errorf("could not initialize stack: %w", err)
		}
	}

	rcv := &RcvQueue {
		logger: log,
		size: size,
		workers: 0,
		max: max,
		codec: codec,
		cache: cache,
		queue: queue,
		stacks: stacks,
	}

	return rcv, nil
}

// Allow us to set the network engines that we will need to process our message
func (r *RcvQueue) SetEngines(engines *map[uint8]network.Engine) {
	r.engines = engines
}

// Determine the numerical priority of our incoming message
func (r *RcvQueue) Priority(v interface {}) int {
	switch v.(type) {
		// consensus
		case *messages.BlockProposal:
			return 10
		case *messages.BlockVote:
			return 10

		// protocol state sync
		case *messages.SyncRequest:
			return 3
		case *messages.SyncResponse:
			return 3
		case *messages.RangeRequest:
			return 3
		case *messages.BatchRequest:
			return 3
		case *messages.BlockResponse:
			return 3

		// cluster consensus
		case *messages.ClusterBlockProposal:
			return 10
		case *messages.ClusterBlockVote:
			return 10
		case *messages.ClusterBlockResponse:
			return 3

		// collections, guarantees & transactions
		case *flow.CollectionGuarantee:
			return 10
		case *flow.TransactionBody:
			return 10
		case *flow.Transaction:
			return 10

		// core messages for execution & verification
		case *flow.ExecutionReceipt:
			return 10
		case *flow.ResultApproval:
			return 10

		// execution state synchronization
		case *messages.ExecutionStateSyncRequest:
			return 5
		case *messages.ExecutionStateDelta:
			return 5

		// data exchange for execution of blocks
		case *messages.ChunkDataRequest:
			return 10
		case *messages.ChunkDataResponse:
			return 10

		// generic entity exchange engines
		case *messages.EntityRequest:
			return 3
		case *messages.EntityResponse:
			return 3

		default:
			return 1
	}
}

// Provides a size score when given a byte size.
func (r *RcvQueue) Size(size int) int {
  switch {
    case size > 50000:
			return 1
		case size > 30000:
			return 2
		case size > 10000:
			return 3
		case size > 6000:
			return 4
		case size > 5000:
			return 5
		case size > 4000:
			return 6
		case size > 3000:
			return 7
		case size > 2000:
			return 8
		case size > 1000:
			return 9
    default:
      return 10
  }
}

// Provides a combined priority and size score when given a message.
func (r *RcvQueue) Score(s int, p interface{}) int {
	size := r.Size(s)
	priority := r.Priority(p)
	return int(math.Ceil(float64((size + priority) / 2)))
}

// Remove the oldest entry in the queue and process it, shift something from the priority lanes and submit it, or die.
func (r *RcvQueue) Work() {
	if r.queue.Len() > 0 {
		key, value, ok := r.queue.GetOldest()

		if ok {
			r.queue.Remove(key)

			// Convert message payload to a known message type
			decodedMessage, decodedErr := r.codec.Decode(value.(RcvQueueValue).message.Payload)
			if decodedErr != nil {
				r.logger.
					Debug().
					Str("sender_id", value.(RcvQueueValue).senderID.String()).
					Str("event_id", key.(RcvQueueKey).eventID).
					Msg(fmt.Sprintf("could not decode event: %w", decodedErr))
				r.workers--
				return
			}

			score := r.Score(value.(RcvQueueValue).message.Size(), decodedMessage)

			stack := RcvStackValue {
				senderID: value.(RcvQueueValue).senderID,
				decodedMessage: decodedMessage,
			}

			evicted := r.stacks[score - 1].Add(key, stack)
			if evicted {
				r.logger.
					Debug().
					Msg("an eviction occured on a priority stack. increase priority stack size.")
			}
		}

		r.Work()
	} else {
		// check priority queues for messages
		work := -1
		for i := 9; i > -1; i-- {
			if r.stacks[i].Len() > 0 {
				work = i
			}
		}

		if work == -1 {
			r.workers--
		} else {
			key, value, ok := r.stacks[work].GetOldest()

			if ok {
				r.stacks[work].Remove(key)

				// Extract channel id and find the registered engine.
				engine, found := (*r.engines)[key.(RcvQueueKey).channelID]
				if !found {
					r.logger.
						Debug().
						Str("sender_id", value.(RcvStackValue).senderID.String()).
						Str("event_id", key.(RcvQueueKey).eventID).
						Msg(fmt.Sprintf("invalid engine error on channel: %d", key.(RcvQueueKey).channelID))
					r.workers--
					return
				}

				// Call the engine asynchronously with the message payload.
				engine.Submit(value.(RcvStackValue).senderID, value.(RcvStackValue).decodedMessage)
			}

			r.Work()
		}
	}
}

// Add adds a new message to the queue if not already present. Returns true if the message was already in the queue or there are
// too many messages in the queue, false otherwise
func (r *RcvQueue) Add(senderID flow.Identifier, message *message.Message) bool {
	key := RcvQueueKey {
		eventID:   string(message.EventID),
		channelID: uint8(message.ChannelID),
	}

	found, _ := r.cache.ContainsOrAdd(key, true)
	// return false if we found a duplicate or our overflow queue is full
	if found || r.queue.Len() == r.size {
		return false
	}

	value := RcvQueueValue {
		senderID: senderID,
		message:  message,
	}
	r.queue.Add(key, value)

	// Start a worker if needed
	if r.workers < r.max {
		r.workers++
		go r.Work()
	}

	return true
}
