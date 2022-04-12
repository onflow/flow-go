package compliance

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

func WithSkipNewProposalsThreshold(threshold uint64) Opt {
	return func(config *Config) {
		config.SkipNewProposalsThreshold = threshold
	}
}
