package signature

// AggregatorMaker the factory for creating SigAggregator
type AggregatorMaker interface {
	NewSigAggregator() SigAggregator
}
