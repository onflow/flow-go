package logical

import (
	"math"
)

// We will use txIndex as logical time for the purpose of block execution.
//
// Execution time refers to the transaction's start time.  Snapshot time refers
// to the time when the snapshot first becomes readable (i.e., the "snapshot
// time - 1" transaction committed the snapshot view).  Each transaction's
// snapshot time must be smaller than or equal to its execution time.
//
// Normal transaction advances the time clock and must be committed to
// DerivedBlockData in monotonically increasing execution time order.
//
// Snapshot read transaction (aka script) does not advance the time clock.  Its
// execution and snapshot time must be set to the latest snapshot time (or
// EndOfBlockExecutionTime in case the real logical time is unavailable).
//
// Note that the "real" txIndex range is [0, math.MaxUint32], but we have
// expanded the range to support events that are not part of the block
// execution.
type Time int64

const (
	// All events associated with the parent block is assigned the same value.
	//
	// Note that we can assign the time to any value in the range
	// [math.MinInt64, -1].
	ParentBlockTime = Time(-1)

	// All events associated with a child block is assigned the same value.
	//
	// Note that we can assign the time to any value in the range
	// (math.MaxUint32 + 1, math.MaxInt64].  (The +1 is needed for assigning
	// EndOfBlockExecutionTime a unique value)
	ChildBlockTime = Time(math.MaxInt64)

	// EndOfBlockExecutionTime is used when the real tx index is unavailable,
	// such as during script execution.
	EndOfBlockExecutionTime = ChildBlockTime - 1

	// A normal transaction cannot commit to EndOfBlockExecutionTime.
	//
	// Note that we can assign the time to any value in the range
	// [max.MathUInt32, EndOfBlockExecutionTime)
	LargestNormalTransactionExecutionTime = EndOfBlockExecutionTime - 1
)
