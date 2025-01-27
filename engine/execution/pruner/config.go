package pruner

import "time"

type PruningConfig struct {
	Threshold                 uint64
	BatchSize                 int
	SleepAfterEachBatchCommit time.Duration
	SleepAfterEachIteration   time.Duration
}

var DefaultConfig = &PruningConfig{
	Threshold:                 30 * 60 * 60 * 24 * 1, // days * hours * minutes * seconds * block_per_second
	BatchSize:                 1000,
	SleepAfterEachBatchCommit: 1 * time.Second,
	SleepAfterEachIteration:   5 * time.Minute,
}
