package epochmgr

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	epochmgr "github.com/dapperlabs/flow-go/engine/collection/epochmgr/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
)

type Suite struct {
	suite.Suite

	// engine dependencies
	me    *module.Local
	state *protocol.State

	// qc voter dependencies
	signer  *hotstuff.Signer
	client  *module.QCContractClient
	voter   *module.ClusterRootQCVoter
	factory *epochmgr.EpochComponentsFactory

	engine *Engine
}

func (suite *Suite) SetupTest() {

	log := zerolog.New(ioutil.Discard)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)

	suite.signer = new(hotstuff.Signer)
	suite.client = new(module.QCContractClient)
	suite.voter = new(module.ClusterRootQCVoter)
	suite.factory = new(epochmgr.EpochComponentsFactory)

	var err error
	suite.engine, err = New(log, suite.me, suite.state, suite.voter, suite.factory)
	suite.Require().Nil(err)
}

func TestEpochManager(t *testing.T) {
	suite.Run(t, new(Suite))
}
