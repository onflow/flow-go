package relay

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// The relay engine copies messages from the staked network to the unstaked network. Additionally, it allows unstaked
// access nodes to register and unregister.
type Engine struct {
	unit *engine.Unit   // used to manage concurrency & shutdown
	log  zerolog.Logger // used to log relevant actions with context
	stakedNetwork module.Network // reference to the staked network
	unstakedNetwork module.Network // reference to the unstaked network
	unstakedConduit network.Conduit
}

func New(
	log zerolog.Logger,
	stakedNetwork module.Network,
	unstakedNetwork module.Network,
) (*Engine, error) {

	relayEngine := &Engine{
		log:        log,
		unit:       engine.NewUnit(),
		stakedNetwork: stakedNetwork,
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
	panic("implement me")
}

func (e Engine) Submit(originID flow.Identifier, event interface{}) {
	panic("implement me")
}

func (e Engine) ProcessLocal(event interface{}) error {
	panic("implement me")
}

func (e Engine) Process(originID flow.Identifier, event interface{}) error {
	panic("implement me")
}
