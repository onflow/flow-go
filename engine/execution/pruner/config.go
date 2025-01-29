package pruner

import "time"

type PruningConfig struct {
	Threshold                 uint64        // The threshold is the number of blocks that we want to keep in the database.
	BatchSize                 uint          // The batch size is the number of blocks that we want to delete in one batch.
	SleepAfterEachBatchCommit time.Duration // The sleep time after each batch commit.
	SleepAfterEachIteration   time.Duration // The sleep time after each iteration.
}

var DefaultConfig = PruningConfig{
	Threshold:                 30 * 60 * 60 * 24 * 1.2, // (30 days of blocks) days * hours * minutes * seconds * block_per_second
	BatchSize:                 1000,
	SleepAfterEachBatchCommit: 1 * time.Second,
	SleepAfterEachIteration:   500000 * time.Hour, // by default it's disabled so that we can slowly roll this feature out.
}
