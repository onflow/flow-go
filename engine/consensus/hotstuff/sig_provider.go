package hotstuff

type SigProvider interface {
	SigVerifier
	Signer
	NewSigAggregator() SigAggregator
}
