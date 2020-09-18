package epochmgr

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/module/epochs"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
)

type Suite struct {
	suite.Suite

	// engine dependencies
	me    *module.Local
	state *protocol.State
	pool  *mempool.Transactions

	// qc voter dependencies
	signer *hotstuff.Signer
	client *module.QCContractClient
	voter  *epochs.RootQCVoter

	engine *Engine
}

func (suite *Suite) SetupTest() {

	log := zerolog.New(ioutil.Discard)
	suite.me = new(module.Local)
	suite.state = new(protocol.State)
	suite.pool = new(mempool.Transactions)

	suite.signer = new(hotstuff.Signer)
	suite.client = new(module.QCContractClient)
	suite.voter = epochs.NewRootQCVoter(log, suite.me, suite.signer, suite.state, suite.client)

	suite.engine = New(log, suite.me, suite.state, suite.pool, suite.voter)
}

func TestEpochManager(t *testing.T) {
	suite.Run(t, new(Suite))
}
