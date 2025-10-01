package backend

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EventsSuite2 struct {
	suite.Suite
}

func TestEventsSuite2(t *testing.T) {
	suite.Run(t, new(EventsSuite2))
}

func (s *EventsSuite2) SetupTest() {

}

func (s *EventsSuite2) HappyPathLocalStorage() {
	s.Run("happy path - all new blocks", func() {

	})

	s.Run("happy path - partial backfill", func() {

	})

	s.Run("happy path - complete backfill", func() {

	})

	s.Run("happy path - start from root block by height", func() {

	})

	s.Run("happy path - start from root block by id", func() {

	})
}

func (s *EventsSuite2) SubscribeFromHeight() {
	//log := unittest.Logger()
	//
	//broadcaster := engine.NewBroadcaster()
	//subscriptionFactory := subscription.NewSubscriptionFactory(
	//	log,
	//	broadcaster,
	//	subscription.DefaultSendTimeout,
	//	subscription.DefaultResponseLimit,
	//	subscription.DefaultSendBufferSize,
	//)
	//
	//executionDataTracker := trackermock.NewExecutionDataTracker(s.T())
	//
	//headers := mock.NewHeaders(s.T())
	//eventsProvider := NewEventsProvider(
	//	log,
	//	headers,
	//)
	//
	//backend := NewEventsBackend(unittest.Logger(), subscriptionFactory, executionDataTracker)
}
