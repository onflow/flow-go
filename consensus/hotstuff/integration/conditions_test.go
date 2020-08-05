package integration

import "time"

type Condition func(*Instance) bool

func RightAway(*Instance) bool {
	return true
}

func ViewFinalized(view uint64) Condition {
	return func(in *Instance) bool {
		return in.forks.FinalizedView() >= view
	}
}

// return true if the number of finalized blocks has reached a certain number
func FinalizedCountsAllReached(instances []*Instance, count int) Condition {
	return func(*Instance) bool {
		for _, in := range instances {
			if len(in.finalized) < count {
				return false
			}
		}
		return true
	}
}

func ViewReached(view uint64) Condition {
	return func(in *Instance) bool {
		return in.pacemaker.CurView() >= view
	}
}

func AfterPriod(dur time.Duration) Condition {
	endTime := time.Now().Add(dur)
	return func(in *Instance) bool {
		return time.Now().After(endTime)
	}
}
