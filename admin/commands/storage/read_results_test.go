package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ReadResultsSuite struct {
	suite.Suite

	command commands.AdminCommand
	state   *protocolmock.State
	results *storagemock.ExecutionResults

	final     *flow.Block
	sealed    *flow.Block
	allBlocks []*flow.Block

	finalResult  *flow.ExecutionResult
	sealedResult *flow.ExecutionResult
	allResults   []*flow.ExecutionResult
}

func TestReadResults(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ReadResultsSuite))
}

func (suite *ReadResultsSuite) SetupTest() {
	suite.state = new(protocolmock.State)
	suite.results = new(storagemock.ExecutionResults)

	var blocks []*flow.Block
	var results []*flow.ExecutionResult

	genesis := unittest.GenesisFixture()
	genesisResult := unittest.ExecutionResultFixture(unittest.WithBlock(genesis))
	blocks = append(blocks, genesis)
	results = append(results, genesisResult)

	sealed := unittest.BlockWithParentFixture(genesis.Header)
	sealedResult := unittest.ExecutionResultFixture(
		unittest.WithBlock(sealed),
		unittest.WithPreviousResult(*genesisResult),
	)
	blocks = append(blocks, sealed)
	results = append(results, sealedResult)

	final := unittest.BlockWithParentFixture(sealed.Header)
	finalResult := unittest.ExecutionResultFixture(
		unittest.WithBlock(final),
		unittest.WithPreviousResult(*sealedResult),
	)
	blocks = append(blocks, final)
	results = append(results, finalResult)

	final = unittest.BlockWithParentFixture(final.Header)
	finalResult = unittest.ExecutionResultFixture(
		unittest.WithBlock(final),
		unittest.WithPreviousResult(*finalResult),
	)
	blocks = append(blocks, final)
	results = append(results, finalResult)

	final = unittest.BlockWithParentFixture(final.Header)
	finalResult = unittest.ExecutionResultFixture(
		unittest.WithBlock(final),
		unittest.WithPreviousResult(*finalResult),
	)
	blocks = append(blocks, final)
	results = append(results, finalResult)

	suite.allBlocks = blocks
	suite.sealed = sealed
	suite.final = final
	suite.allResults = results
	suite.finalResult = finalResult
	suite.sealedResult = sealedResult

	suite.state.On("Final").Return(createSnapshot(final.Header))
	suite.state.On("Sealed").Return(createSnapshot(sealed.Header))
	suite.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protocol.Snapshot {
			for _, block := range blocks {
				if block.ID() == blockID {
					return createSnapshot(block.Header)
				}
			}
			return invalid.NewSnapshot(fmt.Errorf("invalid block ID: %v", blockID))
		},
	)
	suite.state.On("AtHeight", mock.Anything).Return(
		func(height uint64) protocol.Snapshot {
			if int(height) < len(blocks) {
				block := blocks[height]
				return createSnapshot(block.Header)
			}
			return invalid.NewSnapshot(fmt.Errorf("invalid height: %v", height))
		},
	)

	suite.results.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			for _, result := range results {
				if result.ID() == resultID {
					return result
				}
			}
			return nil
		},
		func(resultID flow.Identifier) error {
			for _, result := range results {
				if result.ID() == resultID {
					return nil
				}
			}
			return fmt.Errorf("result %#v not found", resultID)
		},
	)

	suite.results.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.ExecutionResult {
			for _, result := range results {
				if result.BlockID == blockID {
					return result
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, result := range results {
				if result.BlockID == blockID {
					return nil
				}
			}
			return fmt.Errorf("result for block %#v not found", blockID)
		},
	)

	suite.command = NewReadResultsCommand(suite.state, suite.results)
}

func (suite *ReadResultsSuite) TestValidateInvalidResultID() {
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"result": true,
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"result": "",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"result": "uhznms",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"result": "deadbeef",
		},
	}))
	assert.Error(suite.T(), suite.command.Validator(&admin.CommandRequest{
		Data: map[string]interface{}{
			"result": 1,
		},
	}))
}

func (suite *ReadResultsSuite) getResults(reqData map[string]interface{}) []*flow.ExecutionResult {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &admin.CommandRequest{
		Data: reqData,
	}
	require.NoError(suite.T(), suite.command.Validator(req))
	result, err := suite.command.Handler(ctx, req)
	require.NoError(suite.T(), err)

	var results []*flow.ExecutionResult
	data, err := json.Marshal(result)
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), json.Unmarshal(data, &results))

	return results
}

func (suite *ReadResultsSuite) TestHandleFinalBlock() {
	results := suite.getResults(map[string]interface{}{
		"block": "final",
	})
	require.Len(suite.T(), results, 1)
	require.EqualValues(suite.T(), results[0], suite.finalResult)
}

func (suite *ReadResultsSuite) TestHandleSealedBlock() {
	results := suite.getResults(map[string]interface{}{
		"block": "sealed",
	})
	require.Len(suite.T(), results, 1)
	require.EqualValues(suite.T(), results[0], suite.sealedResult)
}

func (suite *ReadResultsSuite) TestHandleBlockHeight() {
	for i, result := range suite.allResults {
		results := suite.getResults(map[string]interface{}{
			"block": float64(i),
		})
		require.Len(suite.T(), results, 1)
		require.EqualValues(suite.T(), results[0], result)
	}
}

func (suite *ReadResultsSuite) TestHandleBlockID() {
	for i, result := range suite.allResults {
		results := suite.getResults(map[string]interface{}{
			"block": suite.allBlocks[i].ID().String(),
		})
		require.Len(suite.T(), results, 1)
		require.EqualValues(suite.T(), results[0], result)
	}
}

func (suite *ReadResultsSuite) TestHandleID() {
	for _, result := range suite.allResults {
		results := suite.getResults(map[string]interface{}{
			"result": result.ID().String(),
		})
		require.Len(suite.T(), results, 1)
		require.EqualValues(suite.T(), results[0], result)
	}
}

func (suite *ReadResultsSuite) TestHandleNExceedsRootBlock() {
	// request by block
	results := suite.getResults(map[string]interface{}{
		"block": "final",
		"n":     float64(len(suite.allResults) + 1),
	})
	require.Len(suite.T(), results, len(suite.allResults))
	require.ElementsMatch(suite.T(), results, suite.allResults)

	// request by result ID
	results = suite.getResults(map[string]interface{}{
		"result": suite.finalResult.ID().String(),
		"n":      float64(len(suite.allResults) + 1),
	})
	require.Len(suite.T(), results, len(suite.allResults))
	require.ElementsMatch(suite.T(), results, suite.allResults)
}
