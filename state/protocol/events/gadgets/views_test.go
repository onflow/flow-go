package gadgets

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/utils/unittest"
)

type viewsMachine struct {
	views         *Views
	callbacks     map[uint64]int // # of callbacks at each view
	calls         int            // incremented each time a callback is invoked
	expectedCalls int            // expected value of calls at any given time
}

func (m *viewsMachine) Init(_ *rapid.T) {
	m.views = NewViews()
	m.callbacks = make(map[uint64]int)
}

func (m *viewsMachine) OnView(t *rapid.T) {
	view := rapid.Uint64().Draw(t, "view").(uint64)
	fmt.Println("draw view", view)
	m.views.OnView(view, func(_ *flow.Header) {
		fmt.Println("invoked callback")
		m.calls++
	})

	count := m.callbacks[view]
	m.callbacks[view] = count + 1
}

func (m *viewsMachine) BlockFinalized(t *rapid.T) {
	view := rapid.Uint64().Draw(t, "view").(uint64)
	fmt.Println("finalize view", view)

	block := unittest.BlockHeaderFixture()
	block.View = view
	m.views.BlockFinalized(&block)

	fmt.Println("machine: ", m.callbacks)
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
	rapid.Check(t, rapid.Run(new(viewsMachine)))
}

func TestViews(t *testing.T) {
	views := NewViews()

	calls := []int{}
	order := 0
	makeCallback := func() events.OnViewCallback {
		corder := order
		order++
		return func(*flow.Header) {
			calls = append(calls, corder)
		}
	}

	for i := 1; i <= 10; i++ {
		views.OnView(uint64(i), makeCallback())
		views.OnView(uint64(i), makeCallback())
	}

	views.OnView(101, makeCallback())

	block := unittest.BlockHeaderFixture()
	block.View = 10
	views.BlockFinalized(&block)

	// ensure callbacks were invoked correctly
	assert.Equal(t, 20, len(calls))

	assert.True(t, sort.IntsAreSorted(calls), "callbacks executed in wrong order")

	// ensure map is cleared appropriately (only view 101 should remain)
	assert.Equal(t, 1, len(views.callbacks))
}
