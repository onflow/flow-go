package queue

import (
	"fmt"
	"math"
	"runtime"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/network"
	jsoncodec "github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// The priority levels currently defined.
const (
	Cap            float64 = 50000
	HighPriority   int     = 10
	MediumPriority int     = 5
	LowPriority    int     = 3
	NoPriority     int     = 1
)

// RcvQueue implements an LRU cache of the received eventIDs that delivered to their engines
type RcvQueue struct {
	logger    zerolog.Logger
	size      int
	codec     network.Codec
	cache     *lru.Cache                // The LRU cache we use for de-duplication.
	queue     *lru.Cache                // The LRU cache we use for overflow.
	stacks    [10]*lru.Cache            // Ten LRU caches we use for priority organization.
	engines   *map[uint8]network.Engine // The engines we need to execute the message.
	collector Collector                 // The pool collector for running our async work.
}

// RcvQueueKey represents a key for the cache and queue
type RcvQueueKey struct {
	eventID   string
	channelID uint8
}

// RcvQueueValue represents a value for the queue
type RcvQueueValue struct {
	senderID flow.Identifier
	message  *message.Message
}

// RcvStackValue represents a value for the queue
type RcvStackValue struct {
	senderID       flow.Identifier
	decodedMessage interface{}
}

// NewRcvQueue creates and returns a new RcvQueue
func NewRcvQueue(log zerolog.Logger, size int) (*RcvQueue, error) {
	// Determine the maximum number of cores we can use for processing
	max := int(math.Min(float64(runtime.GOMAXPROCS(0)), float64(runtime.NumCPU())))

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

	collector := StartDispatcher(max)

	rcv := &RcvQueue{
		logger:    log,
		size:      size,
		codec:     codec,
		cache:     cache,
		queue:     queue,
		stacks:    stacks,
		collector: collector,
	}

	return rcv, nil
}

// Allow us to set the network engines that we will need to process our message
func (r *RcvQueue) SetEngines(engines *map[uint8]network.Engine) {
	r.engines = engines
}

// Determine the numerical priority of our incoming message
// ToDo: Consider moving this Priority method to a PriorityManager type of interface and make it fully decouple from the underlying libp2p.
func (r *RcvQueue) Priority(v interface{}) int {
	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		return HighPriority
	case *messages.BlockVote:
		return HighPriority

	// protocol state sync
	case *messages.SyncRequest:
		return LowPriority
	case *messages.SyncResponse:
		return LowPriority
	case *messages.RangeRequest:
		return LowPriority
	case *messages.BatchRequest:
		return LowPriority
	case *messages.BlockResponse:
		return LowPriority

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return HighPriority
	case *messages.ClusterBlockVote:
		return HighPriority
	case *messages.ClusterBlockResponse:
		return LowPriority

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return HighPriority
	case *flow.TransactionBody:
		return HighPriority
	case *flow.Transaction:
		return HighPriority

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return HighPriority
	case *flow.ResultApproval:
		return HighPriority

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return MediumPriority
	case *messages.ExecutionStateDelta:
		return MediumPriority

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return HighPriority
	case *messages.ChunkDataResponse:
		return HighPriority

	// generic entity exchange engines
	case *messages.EntityRequest:
		return LowPriority
	case *messages.EntityResponse:
		return LowPriority

	default:
		return NoPriority
	}
}

// Provides a size score when given a byte size.
func (r *RcvQueue) Size(size float64) int {
	switch {
	case size > Cap*float64(0.75):
		return NoPriority
	case size > Cap*float64(0.50):
		return LowPriority
	case size > Cap*float64(0.25):
		return MediumPriority
	default:
		return HighPriority
	}
}

// Provides a combined priority and size score when given a message.
func (r *RcvQueue) Score(s int, p interface{}) int {
	size := r.Size(float64(s))
	priority := r.Priority(p)
	return int(math.Ceil(float64((size + priority) / 2)))
}

// Remove the oldest entry in the queue and prioritize it.
func (r *RcvQueue) Prioritize() {
	if r.queue.Len() > 0 {
		key, value, ok := r.queue.RemoveOldest()
		if !ok {
			return
		}

		k, kOk := key.(RcvQueueKey)
		v, vOk := value.(RcvQueueValue)
		if !kOk || !vOk {
			return
		}

		// Convert message payload to a known message type
		decodedMessage, decodedErr := r.codec.Decode(v.message.Payload)
		if decodedErr != nil {
			r.logger.
				Debug().
				Hex("sender_id", logging.ID(v.senderID)).
				Str("event_id", k.eventID).
				Err(decodedErr)
			return
		}

		score := r.Score(v.message.Size(), decodedMessage)

		stack := RcvStackValue{
			senderID:       v.senderID,
			decodedMessage: decodedMessage,
		}

		evicted := r.stacks[score-1].Add(key, stack)
		if evicted {
			r.logger.
				Debug().
				Msg("an eviction occured on a priority stack. increase priority stack size.")
		}

		// start a job to process a message
		r.collector.Work <- Work{r: r, m: "Process"}
	}
}

// Shift a decoded message from the priority lanes and process it.
func (r *RcvQueue) Process() {
	// check priority queues for messages
	work := -1
	for i := 9; i > -1; i-- {
		if r.stacks[i].Len() > 0 {
			work = i
			break
		}
	}
	if work == -1 {
		return
	}

	key, value, ok := r.stacks[work].RemoveOldest()
	if !ok {
		return
	}

	k, kOk := key.(RcvQueueKey)
	v, vOk := value.(RcvStackValue)
	if !kOk || !vOk {
		return
	}

	// Extract channel id and find the registered engine.
	engine, found := (*r.engines)[k.channelID]
	if !found {
		r.logger.
			Debug().
			Hex("sender_id", logging.ID(v.senderID)).
			Str("event_id", k.eventID).
			Msgf("invalid engine error on channel: %d", k.channelID)
		return
	}

	// Call the engine synchronously with the message payload.
	err := engine.Process(v.senderID, v.decodedMessage)
	if err != nil {
		r.logger.
			Debug().
			Hex("sender_id", logging.ID(v.senderID)).
			Str("event_id", k.eventID).
			Err(err)
	}
}

// Add adds a new message to the queue if not already present. Returns false if the message was already in the queue or there are
// too many messages in the queue, true otherwise
func (r *RcvQueue) Add(senderID flow.Identifier, message *message.Message) bool {
	key := RcvQueueKey{
		eventID:   string(message.EventID),
		channelID: uint8(message.ChannelID),
	}

	found, _ := r.cache.ContainsOrAdd(key, true)
	// return false if we found a duplicate or our overflow queue is full
	if found || r.queue.Len() == r.size {
		return false
	}

	value := RcvQueueValue{
		senderID: senderID,
		message:  message,
	}
	r.queue.Add(key, value)

	// start a job to prioritize a message
	r.collector.Work <- Work{r: r, m: "Prioritize"}

	return true
}
