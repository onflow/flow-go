package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type Params struct {
	state *State
}

func (p *Params) ChainID() (flow.ChainID, error) {

	final, err := p.state.Final().Head()
	if err != nil {
		return "", fmt.Errorf("could not get final: %w", err)
	}

	return final.ChainID, nil
}
