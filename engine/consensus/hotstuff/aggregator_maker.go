package hotstuff

type AggregatorMaker interface {
	NewSigAggregator() SigAggregator
}
