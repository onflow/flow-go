package data_providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestBlockHeadersProviderSuite struct {
	BlocksProviderSuite
}

func TestBackendBlockHeadersSuite(t *testing.T) {
	suite.Run(t, new(TestBlockHeadersProviderSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *TestBlockHeadersProviderSuite) SetupTest() {
	s.BlocksProviderSuite.SetupTest()
}

// TestBlockHeadersDataProvider_InvalidArguments tests the behavior of the block headers data provider
// when invalid arguments are provided. It verifies that appropriate errors are returned
// for missing or conflicting arguments.
// This test covers the test cases:
// 1. Missing 'block_status' argument.
// 2. Invalid 'block_status' argument.
// 3. Providing both 'start_block_id' and 'start_block_height' simultaneously.
func (s *TestBlockHeadersProviderSuite) TestBlockHeadersDataProvider_InvalidArguments() {
	ctx := context.Background()
	send := make(chan interface{})

	topic := BlockHeadersTopic

	for _, test := range s.invalidArgumentsTestCases() {
		s.Run(test.name, func() {
			provider, err := NewBlockHeadersDataProvider(ctx, s.log, s.api, topic, test.arguments, send)
			s.Require().Nil(provider)
			s.Require().Error(err)
			s.Require().Contains(err.Error(), test.expectedErrorMsg)
		})
	}
}

// TODO: add tests for responses after the WebsocketController is ready
