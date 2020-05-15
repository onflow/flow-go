package trigger

type Trigger struct {
	viewchanges chan struct{}
}

func New(viewchanges chan struct{}) *Trigger {
	t := &Trigger{
		viewchanges: viewchanges,
	}
	return t
}

func (t *Trigger) ViewChange() error {
	t.viewchanges <- struct{}{}
	return nil
}
