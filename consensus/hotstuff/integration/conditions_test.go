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
