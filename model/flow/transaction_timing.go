package flow

import (
	"time"
)

// TransactionTiming is used to track the timing/durations of a transaction through the system
type TransactionTiming struct {
	TransactionID Identifier
	Received      time.Time
	Finalized     time.Time
	Executed      time.Time
	Sealed        time.Time
}
