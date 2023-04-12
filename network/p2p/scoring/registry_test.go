package scoring_test

import (
	"math"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInit tests when a peer id is queried for the first time by the
// app specific penalty function, the penalty is initialized to the initial state.
func TestInitSpamRecords(t *testing.T) {
	reg, cache := newGossipSubAppSpecificScoreRegistry(t)
	peerID := peer.ID("peer-1")

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// when the app specific penalty function is called for the first time, the penalty should be initialized to the initial state.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, score, scoring.InitAppScoreRecordState().Penalty) // penalty should be initialized to the initial state.

	// the cache should now have the peer id.
	assert.True(t, cache.Has(peerID))
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, record.Penalty, scoring.InitAppScoreRecordState().Penalty) // penalty should be initialized to the initial state.
	assert.Equal(t, record.Decay, scoring.InitAppScoreRecordState().Decay)     // decay should be initialized to the initial state.
}

func TestInitWhenGetGoesFirst(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testInitWhenGetFirst(t, p2p.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testInitWhenGetFirst(t, p2p.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testInitWhenGetFirst(t, p2p.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testInitWhenGetFirst(t, p2p.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
}

// testInitWhenGetFirst tests the state of the app specific penalty transition of the node in this order:
// (1) initially, there is no spam record for the peer id in the cache.
// (2) the app specific penalty function is called for the first time for the peer id, and the spam record is initialized in cache.
// (3) a spam violation is reported for the peer id, causing the spam record to be updated in cache.
// (4) the app specific penalty function is called for the second time for the peer id, and the updated spam record is retrieved from cache.
func testInitWhenGetFirst(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// when the app specific penalty function is called for the first time, the penalty should be initialized to the initial state.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, score, scoring.InitAppScoreRecordState().Penalty) // penalty should be initialized to the initial state.

	// the cache should now have the peer id.
	assert.True(t, cache.Has(peerID))
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, record.Penalty, scoring.InitAppScoreRecordState().Penalty) // penalty should be initialized to the initial state.
	assert.Equal(t, record.Decay, scoring.InitAppScoreRecordState().Decay)     // decay should be initialized to the initial state.

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: messageType,
		Count:   1,
	})

	// the penalty should now be updated.
	record, err, ok = cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)        // penalty should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	// when the app specific penalty function is called again, the penalty should be updated.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty-score), 10e-3) // penalty should be updated to -10.
}

func TestInitWhenReportGoesFirst(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testInitWhenReportGoesFirst(t, p2p.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testInitWhenReportGoesFirst(t, p2p.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testInitWhenReportGoesFirst(t, p2p.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testInitWhenReportGoesFirst(t, p2p.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
}

// testInitWhenReportGoesFirst tests situation where a peer id is reported for the first time for spam violation,
// before the app specific penalty function is called for the first time on it.
// The test expects the penalty to be initialized to the initial state and then updated by the penalty value.
// Subsequent calls to the app specific penalty function should return the updated penalty.
func testInitWhenReportGoesFirst(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: messageType,
		Count:   1,
	})

	// the penalty should now be updated.
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)        // penalty should be updated to -10, we account for decay.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	// when the app specific penalty function is called for the first time, the penalty should be updated.
	// note that since there is a spam penalty, the peer is deprived of the base staked identity reward, and
	// the penalty is only comprised of the spam penalty.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty-score), 10e-3) // penalty should be updated to -10, we account for decay.
}

// TestSpamPenaltyDecaysInCache tests that the spam penalty records decay over time in the cache.
func TestSpamPenaltyDecaysInCache(t *testing.T) {
	peerID := peer.ID("peer-1")
	reg, _ := newGossipSubAppSpecificScoreRegistry(t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgPrune,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgIHave,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgIWant,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	// when the app specific penalty function is called for the first time, the decay functionality should be kicked in
	// the cache, and the penalty should be updated. Note that since the penalty values are negative, the default staked identity
	// reward is not applied. Hence, the penalty is only comprised of the penalties.
	score := reg.AppSpecificScoreFunc()(peerID)
	// the upper bound is the sum of the penalties without decay.
	scoreUpperBound := penaltyValueFixtures().Prune +
		penaltyValueFixtures().Graft +
		penaltyValueFixtures().IHave +
		penaltyValueFixtures().IWant
	// the lower bound is the sum of the penalties with decay assuming the decay is applied 4 times to the sum of the penalties.
	// in reality, the decay is applied 4 times to the first penalty, then 3 times to the second penalty, and so on.
	scoreLowerBound := scoreUpperBound * math.Pow(scoring.InitAppScoreRecordState().Decay, 4)

	// with decay, the penalty should be between the upper and lower bounds.
	assert.Greater(t, score, scoreUpperBound)
	assert.Less(t, score, scoreLowerBound)
}

// TestSpamPenaltyDecayToZero tests that the spam penalty decays to zero over time, and when the spam penalty of
// a peer is set back to zero, its app specific penalty is also reset to the initial state.
func TestSpamPenaltyDecayToZero(t *testing.T) {
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopCollector(), scoring.DefaultDecayFunction())

	// mocks peer has an staked identity and is subscribed to the allowed topics.
	idProvider := mock.NewIdentityProvider(t)
	peerID := peer.ID("peer-1")
	idProvider.On("ByPeerID", peerID).Return(unittest.IdentityFixture(), true).Maybe()

	validator := mockp2p.NewSubscriptionValidator(t)
	validator.On("CheckSubscribedToAllowedTopics", peerID, testifymock.Anything).Return(nil).Maybe()

	reg := scoring.NewGossipSubAppSpecificScoreRegistry(&scoring.GossipSubAppSpecificScoreRegistryConfig{
		Logger:        unittest.Logger(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Penalty:       penaltyValueFixtures(),
		Validator:     validator,
		IdProvider:    idProvider,
		CacheFactory: func() p2p.GossipSubSpamRecordCache {
			return cache
		},
		Init: func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		},
	})

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                      // the penalty should be less than zero.
	require.Greater(t, score, penaltyValueFixtures().Graft) // the penalty should be less than the penalty value due to decay.

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := cache.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should reset back to its initial state (i.e., max reward).
		return reg.AppSpecificScoreFunc()(peerID) == scoring.MaxAppSpecificReward
	}, 5*time.Second, 100*time.Millisecond)

	// the penalty should now be zero.
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.
}

// withStakedIdentity returns a function that sets the identity provider to return an staked identity for the given peer id.
// It is used for testing purposes, and causes the given peer id to benefit from the staked identity reward in GossipSub.
func withStakedIdentity(peerId peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.IdProvider.(*mock.IdentityProvider).On("ByPeerID", peerId).Return(unittest.IdentityFixture(), true).Maybe()
	}
}

// withValidSubscriptions returns a function that sets the subscription validator to return nil for the given peer id.
// It is used for testing purposes and causes the given peer id to never be penalized for subscribing to invalid topics.
func withValidSubscriptions(peer peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Validator.(*mockp2p.SubscriptionValidator).On("CheckSubscribedToAllowedTopics", peer, testifymock.Anything).Return(nil).Maybe()
	}
}

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry(t *testing.T, opts ...func(*scoring.GossipSubAppSpecificScoreRegistryConfig)) (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.GossipSubSpamRecordCache) {
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopCollector(), scoring.DefaultDecayFunction())
	cfg := &scoring.GossipSubAppSpecificScoreRegistryConfig{
		Logger:        unittest.Logger(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Init:          scoring.InitAppScoreRecordState,
		Penalty:       penaltyValueFixtures(),
		IdProvider:    mock.NewIdentityProvider(t),
		Validator:     mockp2p.NewSubscriptionValidator(t),
		CacheFactory: func() p2p.GossipSubSpamRecordCache {
			return cache
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return scoring.NewGossipSubAppSpecificScoreRegistry(cfg), cache
}

// penaltyValueFixtures returns a set of penalty values for testing purposes.
// The values are not realistic. The important thing is that they are different from each other. This is to make sure
// that the tests are not passing because of the default values.
func penaltyValueFixtures() scoring.GossipSubCtrlMsgPenaltyValue {
	return scoring.GossipSubCtrlMsgPenaltyValue{
		Graft: -100,
		Prune: -50,
		IHave: -20,
		IWant: -10,
	}
}
