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
		b := rapid.OneOf(rapid.Just(unittest.BlockHeaderFixture()), rapid.SampledFrom(store)).Draw(t, "parent").(flow.Header)
		store = append(store, unittest.BlockHeaderWithParentFixture(&b))
	}
	return store
}

type rapidSync struct {
	store      []flow.Header
	core       *Core
	idRequests map[flow.Identifier]int // pushdown automaton to track ID requests
}

// Init is an action for initializing a rapidSync instance.
func (r *rapidSync) Init(t *rapid.T) {
	var err error

	r.core, err = New(zerolog.New(ioutil.Discard), DefaultConfig())
	require.NoError(t, err)

	r.store = populatedBlockStore(t)
	r.idRequests = make(map[flow.Identifier]int)
}

// RequestByID is an action that requests a block by its ID.
func (r *rapidSync) RequestByID(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "id_request").(flow.Header)
	r.core.RequestBlock(b.ID())
	// Re-queueing by ID should always succeed
	r.idRequests[b.ID()] = 1
}

// HandleByID is an action that provides a block header to the sync engine
func (r *rapidSync) HandleByID(t *rapid.T) {
	b := rapid.SampledFrom(r.store).Draw(t, "id_handling").(flow.Header)
	success := r.core.HandleBlock(&b)
	assert.True(t, success || r.idRequests[b.ID()] == 0)

	// we decrease the pending requests iff we have already requested this block
	// and we have not received it since
	if r.idRequests[b.ID()] == 1 {
		r.idRequests[b.ID()] = 0
	}
}

// Check runs after every action and verifies that all required invariants hold.
func (r *rapidSync) Check(t *rapid.T) {
	for k, v := range r.idRequests {
		if v == 1 {
			s, ok := r.core.blockIDs[k]
			require.True(t, ok)
			assert.True(t, s.WasQueued(), "ID %v was expected to be Queued and is %v", k, s.StatusString())
		} else if v == 0 {
			s, ok := r.core.blockIDs[k]
			// if a block is known with 0 pendings, it's because it was received
			if ok {
				assert.True(t, s.WasReceived(), "ID %v was expected to be Received and is %v", k, s.StatusString())
			}
		} else {
			t.Fatalf("incorrect management of idRequests in the tests")
		}
	}
}

func TestRapidSync(t *testing.T) {
	rapid.Check(t, rapid.Run(&rapidSync{}))
}
