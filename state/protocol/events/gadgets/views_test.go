package gadgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// viewsMachine is a wrapper around Views which provides conditional actions
// for rapid property-based testing.
type viewsMachine struct {
	views         *Views
	callbacks     map[uint64]int // # of callbacks at each view
	calls         int            // incremented each time a callback is invoked
	expectedCalls int            // expected value of calls at any given time
}

func (m *viewsMachine) init(_ *rapid.T) {
	m.views = NewViews()
	m.callbacks = make(map[uint64]int)
	m.calls = 0
	m.expectedCalls = 0
}

func (m *viewsMachine) OnView(t *rapid.T) {
	view := rapid.Uint64().Draw(t, "view")
	m.views.OnView(view, func(_ *flow.Header) {
		m.calls++ // count actual number of calls invoked by Views
	})

	count := m.callbacks[view]
	m.callbacks[view] = count + 1
}

func (m *viewsMachine) BlockFinalized(t *rapid.T) {
	view := rapid.Uint64().Draw(t, "view")

	block := unittest.BlockHeaderFixture()
	block.View = view
	m.views.BlockFinalized(block)

	// increase the number of expected calls and remove those callbacks from our model
	for indexedView, nCallbacks := range m.callbacks {
		if indexedView <= view {
			m.expectedCalls += nCallbacks
			delete(m.callbacks, indexedView)
		}
	}
}

func (m *viewsMachine) Check(t *rapid.T) {
	// the expected number of callbacks should be invoked
	assert.Equal(t, m.expectedCalls, m.calls)
}

func TestViewsRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sm := new(viewsMachine)
		sm.init(t)
		t.Repeat(rapid.StateMachineActions(sm))
	})
}
