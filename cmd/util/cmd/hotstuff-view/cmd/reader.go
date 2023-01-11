package cmd

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/persister"
)

type Reader struct {
	persister *persister.Persister
}

func NewReader(persister *persister.Persister) *Reader {
	return &Reader{
		persister: persister,
	}
}

func (r *Reader) SetHotstuffView(view uint64) error {
	if view == 0 {
		return fmt.Errorf("hotstuff view is not allowed to set to 0, please specify --view")
	}

	// ensure we don't set the view lower than the current view, because this may cause this node to violate protocol rules
	currentView, err := r.GetHotstuffView()
	if err != nil {
		return err
	}
	if view < currentView {
		return fmt.Errorf("hotstuff view %v cannot be lower than current view (%d)", view, currentView)
	}

	err = r.persister.PutStarted(view)
	if err != nil {
		return fmt.Errorf("could not put hotstuff view %v: %w", view, err)
	}

	return nil
}

func (r *Reader) GetHotstuffView() (uint64, error) {
	view, err := r.persister.GetStarted()
	if err != nil {
		return 0, fmt.Errorf("could not get hotstuff view %w", err)
	}

	return view, nil
}
