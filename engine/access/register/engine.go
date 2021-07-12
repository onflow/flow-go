package relay

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// The register engine allows unstaked access nodes to register and unregister with the staked access node
type Engine struct {
	unit            *engine.Unit   // used to manage concurrency & shutdown
	log             zerolog.Logger // used to log relevant actions with context
	unstakedNetwork module.Network // reference to the unstaked network
	unstakedConduit network.Conduit
}

func New(
	log zerolog.Logger,
	unstakedNetwork module.Network,
) (*Engine, error) {

	relayEngine := &Engine{
		log:             log,
		unit:            engine.NewUnit(),
		unstakedNetwork: unstakedNetwork,
	}

	unstakedConduit, err := unstakedNetwork.Register(engine.ManageUnstakedAccess, relayEngine)
	if err != nil {
		return nil, err
	}

	relayEngine.unstakedConduit = unstakedConduit

	return relayEngine, nil
}

func (e Engine) SubmitLocal(event interface{}) {
	return
}

func (e Engine) Submit(originID flow.Identifier, event interface{}) {
	return
}

func (e Engine) ProcessLocal(event interface{}) error {
	return nil
}

func (e Engine) Process(originID flow.Identifier, event interface{}) error {
	return nil
}

func register(unstakedConduit network.Conduit) error {
	return unstakedConduit.Unicast()
}
