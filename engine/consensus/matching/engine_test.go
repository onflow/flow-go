// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestMatchingEngine(t *testing.T) {
	suite.Run(t, new(MatchingSuite))
}

type MatchingSuite struct {
	suite.Suite

	origin *flow.Identity

	state    *protocol.State
	snapshot *protocol.Snapshot

	me *module.Local

	resultsDB *storage.ExecutionResults
	headersDB *storage.Headers

	resultsPL   *mempool.Results
	receiptsPL  *mempool.Receipts
	approvalsPL *mempool.Approvals
	sealsPL     *mempool.Seals

	matching *Engine
}

func (ms *MatchingSuite) SetupTest() {

	log := zerolog.New(ioutil.Discard)
	metrics := metrics.NewNoopCollector()

	ms.origin = unittest.IdentityFixture()

	ms.state = &protocol.State{}
	ms.state.On("Final").Return(ms.snapshot, nil)
	ms.snapshot = &protocol.Snapshot{}
	ms.snapshot.On("Identity", ms.origin.NodeID).Return(ms.origin, nil)

	ms.matching = &Engine{
		log:       log,
		metrics:   metrics,
		mempool:   metrics,
		state:     ms.state,
		me:        ms.me,
		resultsDB: ms.resultsDB,
		headersDB: ms.headersDB,
		results:   ms.resultsPL,
		receipts:  ms.receiptsPL,
		approvals: ms.approvalsPL,
		seals:     ms.sealsPL,
	}
}

func (ms *MatchingSuite) TestOnReceiptValid() {
}

func (ms *MatchingSuite) TestOnReceiptInvalidOrigin() {
}

func (ms *MatchingSuite) TestOnReceiptInvalidRole() {
}

func (ms *MatchingSuite) TestOnReceiptInvalidStake() {
}

func (ms *MatchingSuite) TestOnReceiptExistingResult() {
}

func (ms *MatchingSuite) TestOnApprovalValid() {
}

func (ms *MatchingSuite) TestOnApprovalInvalidOrigin() {
}

func (ms *MatchingSuite) TestOnApprovalInvalidRole() {
}

func (ms *MatchingSuite) TestOnApprovalInvalidStake() {
}

func (ms *MatchingSuite) TestSealableResultsValid() {
}

func (ms *MatchingSuite) TestSealableResultsNoResults() {
}

func (ms *MatchingSuite) TestSealableResultsNoApprovals() {
}

func (ms *MatchingSuite) TestSealableResultsInsufficientStake() {
}

func (ms *MatchingSuite) TestSealResultValid() {
}

func (ms *MatchingSuite) TestSealResultUnknownBlock() {
}

func (ms *MatchingSuite) TestSealResultUnknownPrevious() {
}
