package data_providers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/state_stream"
	statestreammock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// DataProviderFactorySuite is a test suite for testing the DataProviderFactory functionality.
type DataProviderFactorySuite struct {
	suite.Suite

	ctx context.Context
	ch  chan interface{}

	factory *DataProviderFactory
}

func TestDataProviderFactorySuite(t *testing.T) {
	suite.Run(t, new(DataProviderFactorySuite))
}

// SetupTest sets up the initial context and dependencies for each test case.
// It initializes the factory with mock instances and validates that it is created successfully.
func (s *DataProviderFactorySuite) SetupTest() {
	log := unittest.Logger()
	eventFilterConfig := state_stream.EventFilterConfig{}
	stateStreamApi := statestreammock.NewAPI(s.T())
	accessApi := accessmock.NewAPI(s.T())

	s.ctx = context.Background()
	s.ch = make(chan interface{})

	s.factory = NewDataProviderFactory(log, eventFilterConfig, stateStreamApi, accessApi)
	s.Require().NotNil(s.factory)
}

// TestSupportedTopics verifies that supported topics return a valid provider and no errors.
// Each test case includes a topic and arguments for which a data provider should be created.
func (s *DataProviderFactorySuite) TestSupportedTopics() {

}

// TestUnsupportedTopics verifies that unsupported topics do not return a provider
// and instead return an error indicating the topic is unsupported.
func (s *DataProviderFactorySuite) TestUnsupportedTopics() {
	// Define unsupported topics
	unsupportedTopics := []string{
		"unknown_topic",
		"",
	}

	for _, topic := range unsupportedTopics {
		provider, err := s.factory.NewDataProvider(s.ctx, topic, nil, s.ch)
		s.Require().Nil(provider, "Expected no provider for unsupported topic %s", topic)
		s.Require().Error(err, "Expected error for unsupported topic %s", topic)
		s.Require().EqualError(err, fmt.Sprintf("unsupported topic \"%s\"", topic))
	}
}
