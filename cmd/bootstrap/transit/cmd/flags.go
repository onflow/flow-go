package cmd

import "time"

var (
	flagBootDir       string                            // Bootstrap dir path
	flagBucketName    string = "flow-genesis-bootstrap" // The name of the bucket
	flagToken         string                            // the key directory
	flagAccessAddress string
	flagNodeRole      string
	flagTimeout       time.Duration

	flagWrapID string // wrap ID
)
