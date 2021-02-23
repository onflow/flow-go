package module

// Poker is a pointy stick held by a set of goroutines which they may use to
// poke another set of goroutines to wake them up.
type Poker interface {

	// Poke wakes a sleeping goroutine, if there are any. Otherwise is a no-op. In
	// either case this is non-blocking.
	Poke()
}

// Pokee is a pointy stick that will wake you up when something happened.
type Pokee interface {

	// SomethingHappened returns a channel which will be sent over when
	// something happened.
	SomethingHappened() <-chan struct{}
}

type poker struct {
	pointyStick chan struct{}
}

func NewPoker() (Poker, Pokee) {
	p := &poker{
		pointyStick: make(chan struct{}),
	}
	return p, p
}

func (p *poker) Poke() {
	select {
	case p.pointyStick <- struct{}{}:
	default:
	}
}

func (p *poker) SomethingHappened() <-chan struct{} {
	return p.pointyStick
}
