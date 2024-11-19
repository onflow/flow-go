package data_providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type BlockDigestsProviderSuite struct {
	BlocksProviderSuite
}

func TestBlockDigestsProviderSuite(t *testing.T) {
	suite.Run(t, new(BlockDigestsProviderSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BlockDigestsProviderSuite) SetupTest() {
	s.BlocksProviderSuite.SetupTest()
}

// TestBlockDigestsDataProvider_InvalidArguments tests the behavior of the block digests data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Missing 'block_status' argument.
// 2. Invalid 'block_status' argument.
// 3. Providing both 'start_block_id' and 'start_block_height' simultaneously.
func (s *BlockDigestsProviderSuite) TestBlockDigestsDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlockDigestsTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockDigestsDataProvider(ctx, s.log, s.api, topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TODO: add tests for responses after the WebsocketController is ready
