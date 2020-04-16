package integration

type Condition func(*Instance) bool

func RightAway(*Instance) bool {
	return true
}

func ViewFinalized(view uint64) Condition {
	return func(in *Instance) bool {
		return in.forks.FinalizedView() >= view
	}
}
