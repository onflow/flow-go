package timeoutaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/utils/unittest"
	"testing"
)

func BenchmarkTimeoutAggregator(b *testing.B) {
	createCollector := func(view uint64) (hotstuff.TimeoutCollector, error) {
		clr := timeoutcollector.NewTimeoutCollector(view, nil, nil, nil, nil)
		return clr, nil
	}
	b.Run("no-generic-timeout-collectors", func(b *testing.B) {
		collector := NewTimeoutCollectors(unittest.Logger(), 0, createCollector)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = collector.GetOrCreateCollector(uint64(i))
		}
		b.StopTimer()
	})
	b.Run("generic-timeout-collectors", func(b *testing.B) {
		collector := NewGenericCollectorsImpl(unittest.Logger(), 0, createCollector)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = collector.GetOrCreateCollector(uint64(i))
		}
		b.StopTimer()
	})
	b.Run("no-generic-vote-collector", func(b *testing.B) {
		createCollector := func(view uint64, workers hotstuff.Workers) (hotstuff.VoteCollector, error) {
			clr := votecollector.NewStateMachine(view, unittest.Logger(), nil, nil, nil)
			return clr, nil
		}
		collector := voteaggregator.NewVoteCollectors(unittest.Logger(), 0, nil, createCollector)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = collector.GetOrCreateCollector(uint64(i))
		}
		b.StopTimer()
	})
	b.Run("generic-vote-collector", func(b *testing.B) {
		createCollector := func(view uint64) (hotstuff.VoteCollector, error) {
			clr := votecollector.NewStateMachine(view, unittest.Logger(), nil, nil, nil)
			return clr, nil
		}
		collector := NewGenericCollectorsImpl(unittest.Logger(), 0, createCollector)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, _ = collector.GetOrCreateCollector(uint64(i))
		}
		b.StopTimer()
	})
}
