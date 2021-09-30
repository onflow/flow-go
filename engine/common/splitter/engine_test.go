package splitter_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/splitter"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

func getEvent() interface{} {
	return struct {
		foo string
	}{
		foo: "bar",
	}
}

type Suite struct {
	suite.Suite

	channel network.Channel
	engine  *splitter.Engine
}

func (suite *Suite) SetupTest() {
	suite.channel = network.Channel("test-channel")
	suite.engine = splitter.New(zerolog.Logger{}, suite.channel)
}

func TestSplitter(t *testing.T) {
	suite.Run(t, new(Suite))
}

// TestDownstreamEngineFailure tests the case where one of the engines registered with
// the splitter encounters an error while processing a message.
func (suite *Suite) TestDownstreamEngineFailure() {
	id := unittest.IdentifierFixture()
	event := getEvent()

	engine1 := new(mocknetwork.Engine
	engine2 := new(mocknetwork.Engine

	suite.engine.RegisterEngine(engine1)
	suite.engine.RegisterEngine(engine2)

	processError := errors.New("Process Error!")

	// engine1 processing error should not impact engine2

	engine1.On("Process", suite.channel, id, event).Return(processError).Once()
	engine2.On("Process", suite.channel, id, event).Return(nil).Once()

	err := suite.engine.Process(suite.channel, id, event)
	merr, ok := err.(*multierror.Error)
	suite.Assert().True(ok)
	suite.Assert().Len(merr.Errors, 1)
	suite.Assert().ErrorIs(merr.Errors[0], processError)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 1)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 1)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())

	// engine2 processing error should not impact engine1

	engine1.On("Process", suite.channel, id, event).Return(nil).Once()
	engine2.On("Process", suite.channel, id, event).Return(processError).Once()

	err = suite.engine.Process(suite.channel, id, event)
	merr, ok = err.(*multierror.Error)
	suite.Assert().True(ok)
	suite.Assert().Len(merr.Errors, 1)
	suite.Assert().ErrorIs(merr.Errors[0], processError)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 2)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 2)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
}

// TestProcessUnregisteredChannel tests that receiving a message on an unknown channel
// returns an error.
func (suite *Suite) TestProcessUnknownChannel() {
	id := unittest.IdentifierFixture()
	event := getEvent()

	unknownChannel := network.Channel("unknown-chan")

	engine := new(mocknetwork.Engine

	suite.engine.RegisterEngine(engine)

	err := suite.engine.Process(unknownChannel, id, event)
	suite.Assert().Error(err)

	engine.AssertNumberOfCalls(suite.T(), "Process", 0)
}

// TestConcurrentEvents tests that sending multiple messages concurrently, results in each engine
// receiving every message.
func (suite *Suite) TestConcurrentEvents() {
	id := unittest.IdentifierFixture()
	const numEvents = 10
	const numEngines = 5

	var engines [numEngines]*mocknetwork.Engine

	for i := 0; i < numEngines; i++ {
		engine := new(mocknetwork.Engine
		suite.engine.RegisterEngine(engine)
		engines[i] = engine
	}

	for i := 0; i < numEvents; i++ {
		for _, engine := range engines {
			engine.On("Process", suite.channel, id, i).Return(nil).Once()
		}
	}

	var wg sync.WaitGroup

	for i := 0; i < numEvents; i++ {
		wg.Add(1)

		go func(value int) {
			defer wg.Done()
			err := suite.engine.Process(suite.channel, id, value)
			suite.Assert().Nil(err)
		}(i)
	}

	wg.Wait()

	for _, engine := range engines {
		engine.AssertNumberOfCalls(suite.T(), "Process", numEvents)
		engine.AssertExpectations(suite.T())
	}
}
