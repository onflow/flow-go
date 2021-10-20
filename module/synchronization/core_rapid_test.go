package synchronization

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TODO: remove this restriction and write a proper parametrizable generator (à la SliceOfN) once we have
// a FlatMap combinator in rapid
const NUM_BLOCKS int = 100

// This returns a forest of blocks, some of which are in a parent relationship
// It should include forks
func populatedBlockStore(t *rapid.T) []flow.Header {
	store := []flow.Header{unittest.BlockHeaderFixture()}
	for i := 1; i < NUM_BLOCKS; i++ {
		// we sample from the store 2/3 times to get deeper trees
		b := rapid.OneOf(rapid.Just(unittest.BlockHeaderFixture()), rapid.SampledFrom(store), rapid.SampledFrom(store)).Draw(t, "parent").(flow.Header)
		store = append(store, unittest.BlockHeaderWithParentFixture(&b))
	}
	return store
}

type rapidSync struct {
	store          []flow.Header
	core           *Core
	idRequests     map[flow.Identifier]bool // depth 1 pushdown automaton to track ID requests
	heightRequests map[uint64]bool          // depth 1 pushdown automaton to track height requests
}

// Init is an action for initializing a rapidSync instance.
func (r *rapidSync) Init(t *rapid.T) {
	var err error

	r.core, err = New(zerolog.New(ioutil.Discard), DefaultConfig())
	require.NoError(t, err)

	r.store = populatedBlockStore(t)
	r.idRequests = make(map[flow.Identifier]bool)
	r.heightRequests = make(map[uint64]bool)
}

// RequestByID is an action that requests a block by its ID.
func (r *rapidSync) RequestByID(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "id_request").(flow.Header)
	r.core.RequestBlock(b.ID())
	// Re-queueing by ID should always succeed
	r.idRequests[b.ID()] = true
	// Re-qeueuing by ID "forgets" a past height request
	r.heightRequests[b.Height] = false
}

// RequestByHeight is an action that requests a specific height
func (r *rapidSync) RequestByHeight(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "height_request").(flow.Header)
	r.core.RequestHeight(b.Height)
	// Re-queueing by height should always succeed
	r.heightRequests[b.Height] = true
}

// HandleHeight is an action that requests a heights
// upon receiving an argument beyond a certain tolerance
func (r *rapidSync) HandleHeight(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "height_hint_request").(flow.Header)
	incr := rapid.IntRange(0, (int)(DefaultConfig().Tolerance)+1).Draw(t, "height increment").(int)
	requestHeight := b.Height + (uint64)(incr)
	r.core.HandleHeight(&b, requestHeight)
	// Re-queueing by height should always succeed if beyond tolerance
	if (uint)(incr) > DefaultConfig().Tolerance {
		for h := b.Height + 1; h <= requestHeight; h++ {
			r.heightRequests[h] = true
		}
	}
}

// HandleByID is an action that provides a block header to the sync engine
func (r *rapidSync) HandleByID(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "id_handling").(flow.Header)
	success := r.core.HandleBlock(&b)
	assert.True(t, success || r.idRequests[b.ID()] == false)

	// we decrease the pending requests iff we have already requested this block
	// and we have not received it since
	if r.idRequests[b.ID()] == true {
		r.idRequests[b.ID()] = false
	}
	// we eagerly remove height requests for blocks we receive
	r.heightRequests[b.Height] = false
}

// Check runs after every action and verifies that all required invariants hold.
func (r *rapidSync) Check(t *rapid.T) {
	// we collect the received blocks as determined above
	var receivedBlocks []flow.Header
	// we also collect the pending blocks
	var activeBlocks []flow.Header

	// we check the validity of our pushdown automaton for ID requests and populate activeBlocks / receivedBlocks
	for id, requested := range r.idRequests {
		s, foundID := r.core.blockIDs[id]

		block, foundBlock := findHeader(r.store, func(h flow.Header) bool {
			return h.ID() == id
		})
		require.True(t, foundBlock, "incorrect management of idRequests in the tests: all added IDs are supposed to be from the store")

		if requested {
			require.True(t, foundID, "ID %v is supposed to be known, but isn't", id)

			assert.True(t, s.WasQueued(), "ID %v was expected to be Queued and is %v", id, s.StatusString())
			assert.False(t, s.WasReceived(), "ID %v was expected to be Queued and is %v", id, s.StatusString())
			activeBlocks = append(activeBlocks, *block)
		} else {
			if foundID {
				// if a block is known with 0 pendings, it's because it was received
				assert.True(t, s.WasReceived(), "ID %v was expected to be Received and is %v", id, s.StatusString())
				receivedBlocks = append(receivedBlocks, *block)
			}
		}
	}

	// we collect still-active heights
	var activeHeights []uint64

	for h, requested := range r.heightRequests {
		s, ok := r.core.heights[h]
		if requested {
			require.True(t, ok, "Height %x is supposed to be known, but isn't", h)
			assert.True(t, s.WasQueued(), "Height %x was expected to be Queued and is %v", h, s.StatusString())
			assert.False(t, s.WasReceived(), "Height %x was expected to be Queued and is %v", h, s.StatusString())
			activeHeights = append(activeHeights, h)

		} else {
			// if a height is known with 0 pendings, it's because:
			// - it was received
			// - or because a block at this height was (blockAtHeightWasReceived)
			// - or because a request for a block at that height made us "forget" the prior height reception (clobberedByID)
			if ok {
				wasReceived := s.WasReceived()
				_, blockAtHeightWasReceived := findHeader(receivedBlocks, func(header flow.Header) bool {
					return header.Height == h
				})
				_, clobberedByID := findHeader(activeBlocks, func(header flow.Header) bool {
					return header.Height == h
				})
				heightWasCanceled := wasReceived || blockAtHeightWasReceived || clobberedByID

				assert.True(t, heightWasCanceled, "Height %x was expected to be Received (or filled through a same-height block) and is %v", h, s.StatusString())
			}
		}
	}

	heights, blockIDs := r.core.getRequestableItems()
	// the queueing logic queues intervals, while our r.heightRequests only queues specific requests
	assert.Subset(t, heights, activeHeights, "sync engine's height request tracking lost heights")

	for _, bID := range blockIDs {
		v, ok := r.idRequests[bID]
		require.True(t, ok)
		assert.Equal(t, true, v, "blockID %v is supposed to be pending but is not", bID)
	}
}

func TestRapidSync(t *testing.T) {
	t.Skip("flaky")
	rapid.Check(t, rapid.Run(&rapidSync{}))
}

// utility functions
func findHeader(store []flow.Header, predicate func(flow.Header) bool) (*flow.Header, bool) {
	for _, b := range store {
		if predicate(b) {
			return &b, true
		}
	}
	return nil, false
}
