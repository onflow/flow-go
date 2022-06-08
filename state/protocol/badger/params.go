package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type Params struct {
	state *State
}

func (p *Params) ChainID() (flow.ChainID, error) {

	// retrieve root header
	root, err := p.Root()
	if err != nil {
		return "", fmt.Errorf("could not get root: %w", err)
	}

	return root.ChainID, nil
}

func (p *Params) SporkID() (flow.Identifier, error) {

	var sporkID flow.Identifier
	err := p.state.db.View(operation.RetrieveSporkID(&sporkID))
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get spork id: %w", err)
	}

	return sporkID, nil
}

func (p *Params) ProtocolVersion() (uint, error) {

	var version uint
	err := p.state.db.View(operation.RetrieveProtocolVersion(&version))
	if err != nil {
		return 0, fmt.Errorf("could not get protocol version: %w", err)
	}

	return version, nil
}

func (p *Params) Root() (*flow.Header, error) {

	// retrieve the root height
	var height uint64
	err := p.state.db.View(operation.RetrieveRootHeight(&height))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root height: %w", err)
	}

	// look up root header
	var rootID flow.Identifier
	err = p.state.db.View(operation.LookupBlockHeight(height, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := p.state.headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}

func (p *Params) Seal() (*flow.Seal, error) {

	// retrieve the root height
	var height uint64
	err := p.state.db.View(operation.RetrieveRootHeight(&height))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root height: %w", err)
	}

	// look up root header
	var rootID flow.Identifier
	err = p.state.db.View(operation.LookupBlockHeight(height, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve the root seal
	seal, err := p.state.seals.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root seal: %w", err)
	}

	return seal, nil
}
