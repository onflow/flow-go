package compliance

const MinSkipNewProposalsThreshold = 1000

// Config is shared config for consensus and collection compliance engines, and
// the consensus follower engine.
type Config struct {
	// SkipNewProposalsThreshold defines the threshold where, if we observe a new
	// proposal which is this far behind our local latest finalized, we drop the
	// proposal rather than cache it.
	SkipNewProposalsThreshold uint64
}

func DefaultConfig() Config {
	return Config{
		SkipNewProposalsThreshold: 100_000,
	}
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
