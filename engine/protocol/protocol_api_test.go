package protocol

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"

	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks                 *storagemock.Blocks
	headers                *storagemock.Headers
	collections            *storagemock.Collections
	transactions           *storagemock.Transactions
	receipts               *storagemock.ExecutionReceipts
	results                *storagemock.ExecutionResults
	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	connectionFactory      *backendmock.ConnectionFactory
	chainID                flow.ChainID
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// query the handler for the latest finalized block
	header, err := protocol.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()

}
