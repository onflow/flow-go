package integration

type Condition func(*Instance) bool

func RightAway(*Instance) bool {
	return true
}

func ViewReached(view uint64) Condition {
	return func(in *Instance) bool {
		return in.pacemaker.CurView() >= view
	}
}
