package synchronization

type Alsp struct {
	syncRequestProbabilityFactor float32
}

func NewAlsp() *Alsp {
	return &Alsp{
		// create misbehavior report 1/1000 times
		syncRequestProbabilityFactor: 0.001,
	}
}
