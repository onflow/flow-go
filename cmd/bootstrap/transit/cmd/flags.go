package cmd

import "time"

var (
	flagBootDir       string                            // Bootstrap dir path
	flagBucketName    string = "flow-genesis-bootstrap" // The name of the bucket
	flagToken         string                            // the key directory
	flagAccessAddress string
	flagNodeRole      string
	flagTimeout       time.Duration
	flagConcurrency   int64

	flagWrapID   string // wrap ID
	flagVoteFile string
)
