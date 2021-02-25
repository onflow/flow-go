package poker

type Poker interface {

	// Poke will wake a goroutine waiting on the wait channel.
	Poke()

	// Wait returns a channel that may be blocked on until poked.
	Wait() <-chan struct{}
}

type poker struct {
	poke chan struct{}
}

func New() Poker {
	p := &poker{
		poke: make(chan struct{}),
	}
	return p
}

func (p *poker) Poke() {
	select {
	case p.poke <- struct{}{}:
	default:
	}
}

func (p *poker) Wait() <-chan struct{} {
	return p.poke
}
