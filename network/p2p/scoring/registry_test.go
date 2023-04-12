package scoring_test

import (
	"fmt"
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

// TestNoPenaltyRecord tests that if there is no penalty record for a peer id, the app specific score should be the max
// app specific reward. This is the default reward for a staked peer that has valid subscriptions and has not been
// penalized.
func TestNoPenaltyRecord(t *testing.T) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	score := reg.AppSpecificScoreFunc()(peerID)
	// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	assert.Equal(t, scoring.MaxAppSpecificReward, score)

	// still the cache should not have the peer id (as there is no spam record for the peer id).
	assert.False(t, cache.Has(peerID))
}

// TestPeerWithSpamRecord tests the app specific penalty computation of the node when there is a spam record for the peer id.
// It tests the state that a staked peer with a valid role and valid subscriptions has spam records.
// Since the peer has spam records, it should be deprived of the default reward for its staked role, and only have the
// penalty value as the app specific score.
func TestPeerWithSpamRecord(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2p.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2p.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2p.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2p.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
}

func testPeerWithSpamRecord(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, scoring.MaxAppSpecificReward, score)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: messageType,
		Count:   1,
	})

	// the penalty should now be updated in the cache
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)        // penalty should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
	// and the peer should be deprived of the default reward for its valid staked role.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty-score), 10e-3)
}

func TestSpamRecord_With_UnknownIdentity(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2p.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2p.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2p.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2p.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
}

// testSpamRecordWithUnknownIdentity tests the app specific penalty computation of the node when there is a spam record for the peer id and
// the peer id has an unknown identity.
func testSpamRecordWithUnknownIdentity(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withUnknownIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// peer does not have spam record, but has an unknown identity. Hence, the app specific score should be the staking penalty.
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Equal(t, scoring.DefaultUnknownIdentityPenalty, score)

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

	// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
	// and the staking penalty.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty+scoring.DefaultUnknownIdentityPenalty-score), 10e-3)
}

func TestSpamRecord_With_SubscriptionPenalty(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2p.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2p.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2p.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2p.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
}

// testSpamRecordWithUnknownIdentity tests the app specific penalty computation of the node when there is a spam record for the peer id and
// the peer id has an invalid subscription as well.
func testSpamRecordWithSubscriptionPenalty(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, cache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withInvalidSubscriptions(peerID))

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// peer does not have spam record, but has invalid subscription. Hence, the app specific score should be subscription penalty.
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Equal(t, scoring.DefaultInvalidSubscriptionPenalty, score)

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
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
	// and the staking penalty.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty+scoring.DefaultInvalidSubscriptionPenalty-score), 10e-3)
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
		// when the spam penalty is decayed to zero, the app specific penalty of the node should reset back to default staking reward.
		return reg.AppSpecificScoreFunc()(peerID) == scoring.DefaultStakedIdentityReward
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

// withUnknownIdentity returns a function that sets the identity provider to return an error for the given peer id.
// It is used for testing purposes, and causes the given peer id to be penalized for not having a staked identity.
func withUnknownIdentity(peer peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.IdProvider.(*mock.IdentityProvider).On("ByPeerID", peer).Return(nil, false).Maybe()
	}
}

// withInvalidSubscriptions returns a function that sets the subscription validator to return an error for the given peer id.
// It is used for testing purposes and causes the given peer id to be penalized for subscribing to invalid topics.
func withInvalidSubscriptions(peer peer.ID) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Validator.(*mockp2p.SubscriptionValidator).On("CheckSubscribedToAllowedTopics", peer, testifymock.Anything).Return(fmt.Errorf("invalid subscriptions")).Maybe()
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
