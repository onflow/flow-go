package splitter_test

import (
	"errors"
	"testing"
	"time"

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

	engine1 := new(mockmodule.Engine)
	engine2 := new(mockmodule.Engine)

	err := suite.engine.RegisterEngine(engine1)
	suite.Assert().Nil(err)
	err = suite.engine.RegisterEngine(engine2)
	suite.Assert().Nil(err)

	// engine1 processing error should not impact engine2

	engine1.On("Process", suite.channel, id, event).Return(errors.New("Process Error!")).Once()
	engine2.On("Process", suite.channel, id, event).Return(nil).Once()

	err = suite.engine.Process(suite.channel, id, event)
	suite.Assert().Nil(err)

	engine1.AssertNumberOfCalls(suite.T(), "Process", 1)
	engine2.AssertNumberOfCalls(suite.T(), "Process", 1)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())

	// engine2 processing error should not impact engine1

	engine1.On("Process", suite.channel, id, event).Return(nil).Once()
	engine2.On("Process", suite.channel, id, event).Return(errors.New("Process Error!")).Once()

	err = suite.engine.Process(suite.channel, id, event)
	suite.Assert().Nil(err)

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

	engine := new(mockmodule.Engine)

	err := suite.engine.RegisterEngine(engine)
	suite.Assert().Nil(err)

	err = suite.engine.Process(unknownChannel, id, event)
	suite.Assert().Error(err)

	engine.AssertNumberOfCalls(suite.T(), "Process", 0)
}

// TestDuplicateRegistrations tests that an engine cannot register for the same channel twice.
func (suite *Suite) TestDuplicateRegistrations() {
	engine := new(mockmodule.Engine)

	err := suite.engine.RegisterEngine(engine)
	suite.Assert().Nil(err)

	err = suite.engine.RegisterEngine(engine)
	suite.Assert().Error(err)
}

// TestReady tests that the splitter's Ready channel closes once all
// registered engines are ready.
func (suite *Suite) TestReady() {
	engine1 := new(mockmodule.Engine)
	engine2 := new(mockmodule.Engine)

	err := suite.engine.RegisterEngine(engine1)
	suite.Assert().Nil(err)
	err = suite.engine.RegisterEngine(engine2)
	suite.Assert().Nil(err)

	ready1 := make(chan struct{})
	ready2 := make(chan struct{})

	engine1.On("Ready").Return((<-chan struct{})(ready1)).Once()
	engine2.On("Ready").Return((<-chan struct{})(ready2)).Once()

	splitterReady := suite.engine.Ready()
	<-time.After(100 * time.Millisecond)

	select {
	case <-splitterReady:
		suite.FailNow("Splitter should not be ready until all registered engines are.")
	default:
	}

	close(ready1)
	<-time.After(100 * time.Millisecond)

	select {
	case <-splitterReady:
		suite.FailNow("Splitter should not be ready until all registered engines are.")
	default:
	}

	close(ready2)

	_, ok := <-splitterReady
	suite.Assert().False(ok)

	engine1.AssertExpectations(suite.T())
	engine2.AssertExpectations(suite.T())
}
