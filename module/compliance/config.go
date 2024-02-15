package compliance

const MinSkipNewProposalsThreshold = 1000

// Config is shared config for consensus and collection compliance engines, and
// the consensus follower engine.
type Config struct {
	// SkipNewProposalsThreshold defines the threshold for dropping blocks that are too far in
	// the future. Formally, let `H` be the view of the latest finalized block known to this
	// node. A new block `B` is dropped without further processing, if
	//   B.View > H + SkipNewProposalsThreshold
	SkipNewProposalsThreshold uint64
}

func DefaultConfig() Config {
	return Config{
		SkipNewProposalsThreshold: 100_000,
	}
}

// GetSkipNewProposalsThreshold returns stored value in config possibly applying a lower bound.
func (c *Config) GetSkipNewProposalsThreshold() uint64 {
	if c.SkipNewProposalsThreshold < MinSkipNewProposalsThreshold {
		return MinSkipNewProposalsThreshold
	}

	return c.SkipNewProposalsThreshold
}

type Opt func(*Config)

// WithSkipNewProposalsThreshold returns an option to set the skip new proposals
// threshold. For inputs less than the minimum threshold, the minimum threshold
// will be set instead.
func WithSkipNewProposalsThreshold(threshold uint64) Opt {
	return func(config *Config) {
		// sanity check: small values are dangerous and can cause finalization halt
		if threshold < MinSkipNewProposalsThreshold {
			threshold = MinSkipNewProposalsThreshold
		}
		config.SkipNewProposalsThreshold = threshold
	}
}
