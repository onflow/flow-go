package unittest

import (
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func EmptyView() *state.View {
	view := state.NewView(func(key flow.RegisterID) (bytes []byte, e error) {
		return nil, nil
	})

	bootstrap.BootstrapView(view) //create genesis state

	return view.NewChild() //return new view
}
