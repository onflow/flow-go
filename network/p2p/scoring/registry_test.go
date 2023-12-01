package scoring_test

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/network/p2p"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNoPenaltyRecord tests that if there is no penalty record for a peer id, the app specific score should be the max
// app specific reward. This is the default reward for a staked peer that has valid subscriptions and has not been
// penalized.
func TestNoPenaltyRecord(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the spamRecords should not have the peer id.
	assert.False(t, spamRecords.Has(peerID))

	score := reg.AppSpecificScoreFunc()(peerID)
	// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	assert.Equal(t, scoring.MaxAppSpecificReward, score)

	// still the spamRecords should not have the peer id (as there is no spam record for the peer id).
	assert.False(t, spamRecords.Has(peerID))
}

// TestPeerWithSpamRecord tests the app specific penalty computation of the node when there is a spam record for the peer id.
// It tests the state that a staked peer with a valid role and valid subscriptions has spam records.
// Since the peer has spam records, it should be deprived of the default reward for its staked role, and only have the
// penalty value as the app specific score.
func TestPeerWithSpamRecord(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testPeerWithSpamRecord(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

func testPeerWithSpamRecord(t *testing.T, messageType p2pmsg.ControlMessageType, basePenalty float64) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the spamRecords should not have the peer id.
	assert.False(t, spamRecords.Has(peerID))

	errSeverity := randomErrSeverity()
	// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, scoring.MaxAppSpecificReward, score)
	expectedPenalty := float64(errSeverity) * basePenalty
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	// the penalty should now be updated in the spamRecords
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
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
		testSpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testSpamRecordWithUnknownIdentity(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

// testSpamRecordWithUnknownIdentity tests the app specific penalty computation of the node when there is a spam record for the peer id and
// the peer id has an unknown identity.
func testSpamRecordWithUnknownIdentity(t *testing.T, messageType p2pmsg.ControlMessageType, basePenalty float64) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withUnknownIdentity(peerID),
		withValidSubscriptions(peerID))

	// initially, the spamRecords should not have the peer id.
	assert.False(t, spamRecords.Has(peerID))
	errSeverity := randomErrSeverity()
	// peer does not have spam record, but has an unknown identity. Hence, the app specific score should be the staking penalty.
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Equal(t, scoring.DefaultUnknownIdentityPenalty, score)
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})
	expectedPenalty := basePenalty * float64(errSeverity)
	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
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
		testSpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testSpamRecordWithSubscriptionPenalty(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

// testSpamRecordWithUnknownIdentity tests the app specific penalty computation of the node when there is a spam record for the peer id and
// the peer id has an invalid subscription as well.
func testSpamRecordWithSubscriptionPenalty(t *testing.T, messageType p2pmsg.ControlMessageType, basePenalty float64) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withInvalidSubscriptions(peerID))

	// initially, the spamRecords should not have the peer id.
	assert.False(t, spamRecords.Has(peerID))

	// peer does not have spam record, but has invalid subscription. Hence, the app specific score should be subscription penalty.
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Equal(t, scoring.DefaultInvalidSubscriptionPenalty, score)
	errSeverity := randomErrSeverity()
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})
	expectedPenalty := basePenalty * float64(errSeverity)
	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
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
	peerID := unittest.PeerIdFixture(t)
	reg, _ := newGossipSubAppSpecificScoreRegistry(t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))
	errSeverity := randomErrSeverity()
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgPrune,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIHave,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIWant,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.RpcPublishMessage,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	// when the app specific penalty function is called for the first time, the decay functionality should be kicked in
	// the cache, and the penalty should be updated. Note that since the penalty values are negative, the default staked identity
	// reward is not applied. Hence, the penalty is only comprised of the penalties.
	score := reg.AppSpecificScoreFunc()(peerID)
	// the upper bound is the sum of the penalties without decay.
	scoreUpperBound := (penaltyValueFixtures().Prune * float64(errSeverity)) +
		(penaltyValueFixtures().Graft * float64(errSeverity)) +
		(penaltyValueFixtures().IHave * float64(errSeverity)) +
		(penaltyValueFixtures().IWant * float64(errSeverity)) +
		(penaltyValueFixtures().RpcPublishMessage * float64(errSeverity))
	// the lower bound is the sum of the penalties with decay assuming the decay is applied 4 times to the sum of the penalties.
	// in reality, the decay is applied 4 times to the first penalty, then 3 times to the second penalty, and so on.
	r := scoring.InitAppScoreRecordState()
	scoreLowerBound := scoreUpperBound * math.Pow(r.Decay, 4)

	// with decay, the penalty should be between the upper and lower bounds.
	assert.Greater(t, score, scoreUpperBound)
	assert.Less(t, score, scoreLowerBound)
}

// TestSpamPenaltyDecayToZero tests that the spam penalty decays to zero over time, and when the spam penalty of
// a peer is set back to zero, its app specific penalty is also reset to the initial state.
func TestSpamPenaltyDecayToZero(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID),
		withInitFunction(func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		}))

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), randomErrSeverity(), p2p.CtrlMsgNonClusterTopicType)},
	})

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                      // the penalty should be less than zero.
	require.Greater(t, score, penaltyValueFixtures().Graft) // the penalty should be less than the penalty value due to decay.

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should reset back to default staking reward.
		return reg.AppSpecificScoreFunc()(peerID) == scoring.DefaultStakedIdentityReward
	}, 5*time.Second, 100*time.Millisecond)

	// the penalty should now be zero.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.
}

// TestPersistingUnknownIdentityPenalty tests that even though the spam penalty is decayed to zero, the unknown identity penalty
// is persisted. This is because the unknown identity penalty is not decayed.
func TestPersistingUnknownIdentityPenalty(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withUnknownIdentity(peerID), // the peer id has an unknown identity.
		withValidSubscriptions(peerID),
		withInitFunction(func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		}))

	// initially, the app specific score should be the default unknown identity penalty.
	require.Equal(t, scoring.DefaultUnknownIdentityPenalty, reg.AppSpecificScoreFunc()(peerID))

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
	})

	// with reported spam, the app specific score should be the default unknown identity + the spam penalty.
	diff := math.Abs(scoring.DefaultUnknownIdentityPenalty + penaltyValueFixtures().Graft - reg.AppSpecificScoreFunc()(peerID))
	normalizedDiff := diff / (scoring.DefaultUnknownIdentityPenalty + penaltyValueFixtures().Graft)
	require.NotZero(t, normalizedDiff, "difference between the expected and actual app specific score should not be zero")
	require.Less(t,
		normalizedDiff,
		0.01, "normalized difference between the expected and actual app specific score should be less than 1%")

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                                                            // the penalty should be less than zero.
	require.Greater(t, score, penaltyValueFixtures().Graft+scoring.DefaultUnknownIdentityPenalty) // the penalty should be less than the penalty value due to decay.

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should only contain the unknown identity penalty.
		return reg.AppSpecificScoreFunc()(peerID) == scoring.DefaultUnknownIdentityPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// the spam penalty should now be zero in spamRecords.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.
}

// TestPersistingInvalidSubscriptionPenalty tests that even though the spam penalty is decayed to zero, the invalid subscription penalty
// is persisted. This is because the invalid subscription penalty is not decayed.
func TestPersistingInvalidSubscriptionPenalty(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withInvalidSubscriptions(peerID), // the peer id has an invalid subscription.
		withInitFunction(func() p2p.GossipSubSpamRecord {
			return p2p.GossipSubSpamRecord{
				Decay:   0.02, // we choose a small decay value to speed up the test.
				Penalty: 0,
			}
		}))

	// initially, the app specific score should be the default invalid subscription penalty.
	require.Equal(t, scoring.DefaultUnknownIdentityPenalty, reg.AppSpecificScoreFunc()(peerID))
	errSeverity := randomErrSeverity()
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})
	expectedPenalty := penaltyValueFixtures().Graft * float64(errSeverity)
	// with reported spam, the app specific score should be the default invalid subscription penalty + the spam penalty.
	require.Less(t, math.Abs(scoring.DefaultInvalidSubscriptionPenalty+expectedPenalty-reg.AppSpecificScoreFunc()(peerID)), 10e-3)

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                                                   // the penalty should be less than zero.
	require.Greater(t, score, expectedPenalty+scoring.DefaultInvalidSubscriptionPenalty) // the penalty should be less than the penalty value due to decay.

	require.Eventually(t, func() bool {
		// the spam penalty should eventually decay to zero.
		r, err, ok := spamRecords.Get(peerID)
		return ok && err == nil && r.Penalty == 0.0
	}, 5*time.Second, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		// when the spam penalty is decayed to zero, the app specific penalty of the node should only contain the default invalid subscription penalty.
		return reg.AppSpecificScoreFunc()(peerID) == scoring.DefaultUnknownIdentityPenalty
	}, 5*time.Second, 100*time.Millisecond)

	// the spam penalty should now be zero in spamRecords.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Penalty) // penalty should be zero.
}

// TestSpamRecordDecayAdjustment ensures that spam record decay is increased each time a peers score reaches the scoring.IncreaseDecayThreshold eventually
// sustained misbehavior will result in the spam record decay reaching the minimum decay speed .99, and the decay speed is reset to the max decay speed .8.
func TestSpamRecordDecayAdjustment(t *testing.T) {
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	scoringRegistryConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubScoringRegistryConfig
	// increase configured DecayRateReductionFactor so that the decay time is increased faster
	scoringRegistryConfig.DecayRateReductionFactor = .1
	scoringRegistryConfig.PenaltyDecayEvaluationPeriod = time.Second

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	reg, spamRecords := newScoringRegistry(
		t,
		scoringRegistryConfig,
		withStakedIdentity(peer1),
		withValidSubscriptions(peer1),
		withStakedIdentity(peer2),
		withValidSubscriptions(peer2))

	// initially, the spamRecords should not have the peer ids.
	assert.False(t, spamRecords.Has(peer1))
	assert.False(t, spamRecords.Has(peer2))
	// since the both peers do not have a spam record, their app specific score should be the max app specific reward, which
	// is the default reward for a staked peer that has valid subscriptions.
	assert.Equal(t, scoring.MaxAppSpecificReward, reg.AppSpecificScoreFunc()(peer1))
	assert.Equal(t, scoring.MaxAppSpecificReward, reg.AppSpecificScoreFunc()(peer2))

	// simulate sustained malicious activity from peer1, eventually the decay speed
	// for a spam record should be reduced to the MinimumSpamPenaltyDecayFactor
	prevDecay := scoring.MaximumSpamPenaltyDecayFactor
	tolerance := 0.1
	require.Eventually(t, func() bool {
		errCount := 500
		errs := make(p2p.InvCtrlMsgErrs, errCount)
		for i := 0; i < errCount; i++ {
			errs[i] = p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CriticalErrSeverity, p2p.CtrlMsgNonClusterTopicType)
		}
		reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  peer1,
			MsgType: p2pmsg.CtrlMsgPrune,
			Errors:  errs,
		})
		record, err, ok := spamRecords.Get(peer1)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Less(t, math.Abs(prevDecay-record.Decay), tolerance)
		prevDecay = record.Decay
		return record.Decay == scoring.MinimumSpamPenaltyDecayFactor
	}, 5*time.Second, 500*time.Millisecond)

	// initialize a spam record for peer2
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peer2,
		MsgType: p2pmsg.CtrlMsgPrune,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CriticalErrSeverity, p2p.CtrlMsgNonClusterTopicType)},
	})
	// reduce penalty and increase Decay to scoring.MinimumSpamPenaltyDecayFactor
	record, err := spamRecords.Update(peer2, func(record p2p.GossipSubSpamRecord) p2p.GossipSubSpamRecord {
		record.Penalty = -.1
		record.Decay = scoring.MinimumSpamPenaltyDecayFactor
		return record
	})
	require.NoError(t, err)
	require.True(t, record.Decay == scoring.MinimumSpamPenaltyDecayFactor)
	require.True(t, record.Penalty == -.1)
	// simulate sustained good behavior from peer 2, each time the spam record is read from the cache
	// using Get method the record penalty will be decayed until it is eventually reset to
	// 0 at this point the decay speed for the record should be reset to MaximumSpamPenaltyDecayFactor
	// eventually after penalty reaches the skipDecaThreshold the record decay will be reset to scoring.MaximumSpamPenaltyDecayFactor
	require.Eventually(t, func() bool {
		record, err, ok := spamRecords.Get(peer2)
		require.NoError(t, err)
		require.True(t, ok)
		return record.Decay == scoring.MaximumSpamPenaltyDecayFactor &&
			record.Penalty == 0 &&
			record.LastDecayAdjustment.IsZero()
	}, 5*time.Second, time.Second)

	// ensure decay can be reduced again after recovery for peerID 2
	require.Eventually(t, func() bool {
		reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  peer2,
			MsgType: p2pmsg.CtrlMsgPrune,
			Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.CriticalErrSeverity, p2p.CtrlMsgNonClusterTopicType)},
		})
		record, err, ok := spamRecords.Get(peer1)
		require.NoError(t, err)
		require.True(t, ok)
		return record.Decay == scoring.MinimumSpamPenaltyDecayFactor
	}, 5*time.Second, 500*time.Millisecond)
}

// TestPeerSpamPenaltyClusterPrefixed evaluates the application-specific penalty calculation for a node when a spam record is present
// for cluster-prefixed topics. In the case of an invalid control message notification marked as cluster-prefixed,
// the application-specific penalty should be reduced by the default reduction factor. This test verifies the accurate computation
// of the application-specific score under these conditions.
func TestPeerSpamPenaltyClusterPrefixed(t *testing.T) {
	ctlMsgTypes := p2pmsg.ControlMessageTypes()
	peerIds := unittest.PeerIdFixtures(t, len(ctlMsgTypes))
	opts := make([]scoringRegistryParamsOpt, 0)
	for _, peerID := range peerIds {
		opts = append(opts, withStakedIdentity(peerID), withValidSubscriptions(peerID))
	}
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(t, opts...)

	for _, peerID := range peerIds {
		// initially, the spamRecords should not have the peer id.
		assert.False(t, spamRecords.Has(peerID))
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		score := reg.AppSpecificScoreFunc()(peerID)
		assert.Equal(t, scoring.MaxAppSpecificReward, score)
	}

	// Report consecutive misbehavior's for the specified peer ID. Two misbehavior's are reported concurrently:
	// 1. With IsClusterPrefixed set to false, ensuring the penalty applied to the application-specific score is not reduced.
	// 2. With IsClusterPrefixed set to true, reducing the penalty added to the overall app-specific score by the default reduction factor.
	for i, ctlMsgType := range ctlMsgTypes {
		errSeverity := randomErrSeverity()
		peerID := peerIds[i]
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
				PeerID:  peerID,
				MsgType: ctlMsgType,
				Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf(fmt.Sprintf("invalid %s", ctlMsgType)), errSeverity, p2p.CtrlMsgNonClusterTopicType)},
			})
		}()
		go func() {
			defer wg.Done()
			reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
				PeerID:  peerID,
				MsgType: ctlMsgType,
				Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf(fmt.Sprintf("invalid %s", ctlMsgType)), errSeverity, p2p.CtrlMsgTopicTypeClusterPrefixed)},
			})
		}()
		unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "timed out waiting for goroutines to finish")

		// expected penalty should be (penaltyValueFixtures().Graft* errSeverity) * (1  + clusterReductionFactor)
		expectedPenalty := (penaltyValueFixture(ctlMsgType) * float64(errSeverity)) * (1 + penaltyValueFixtures().ClusterPrefixedPenaltyReductionFactor)

		// the penalty should now be updated in the spamRecords
		record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
		assert.True(t, ok)
		assert.NoError(t, err)
		assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)
		assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay)
		// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
		// and the peer should be deprived of the default reward for its valid staked role.
		score := reg.AppSpecificScoreFunc()(peerID)
		tolerance := 10e-3 // 0.1%
		if expectedPenalty == 0 {
			assert.Less(t, math.Abs(expectedPenalty), tolerance)
		} else {
			assert.Less(t, math.Abs(expectedPenalty-score)/expectedPenalty, tolerance)
		}
	}
}

// TestInvalidControlMessageMultiErrorScoreCalculation tests that invalid control message penalties are calculated as expected when notifications
// contain multiple errors with multiple different severity levels.
func TestInvalidControlMessageMultiErrorScoreCalculation(t *testing.T) {
	peerIds := unittest.PeerIdFixtures(t, 4)
	opts := make([]scoringRegistryParamsOpt, 0)
	for _, peerID := range peerIds {
		opts = append(opts, withStakedIdentity(peerID), withValidSubscriptions(peerID))
	}
	reg, spamRecords := newGossipSubAppSpecificScoreRegistry(t, opts...)
	for _, peerID := range peerIds {
		// initially, the spamRecords should not have the peer id.
		assert.False(t, spamRecords.Has(peerID))
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		score := reg.AppSpecificScoreFunc()(peerID)
		assert.Equal(t, scoring.MaxAppSpecificReward, score)
	}

	type testCase struct {
		notification    *p2p.InvCtrlMsgNotif
		expectedPenalty float64
	}
	penaltyValues := penaltyValueFixtures()
	testCases := []*testCase{
		// single error with, with random severity
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[0],
				MsgType: p2pmsg.CtrlMsgGraft,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid graft"), p2p.HighErrSeverity, p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: penaltyValues.Graft * float64(p2p.HighErrSeverity),
		},
		// multiple errors with, with same severity
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[1],
				MsgType: p2pmsg.CtrlMsgPrune,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.ModerateErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.ModerateErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid prune"), p2p.ModerateErrSeverity, p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: (penaltyValues.Prune * float64(p2p.ModerateErrSeverity)) * 3,
		},
		// multiple errors with, with random severity's
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[2],
				MsgType: p2pmsg.CtrlMsgIHave,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.CriticalErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.HighErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.ModerateErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid ihave"), p2p.LowErrSeverity, p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: (penaltyValues.IHave * float64(p2p.CriticalErrSeverity)) +
				(penaltyValues.IHave * float64(p2p.HighErrSeverity)) +
				(penaltyValues.IHave * float64(p2p.ModerateErrSeverity)) +
				(penaltyValues.IHave * float64(p2p.LowErrSeverity)),
		},
		// multiple errors with, with random severity's iwant
		{
			notification: &p2p.InvCtrlMsgNotif{
				PeerID:  peerIds[3],
				MsgType: p2pmsg.CtrlMsgIWant,
				Errors: p2p.InvCtrlMsgErrs{
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.CriticalErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.HighErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.ModerateErrSeverity, p2p.CtrlMsgNonClusterTopicType),
					p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid iwant"), p2p.LowErrSeverity, p2p.CtrlMsgNonClusterTopicType),
				},
			},
			expectedPenalty: (penaltyValues.IWant * float64(p2p.CriticalErrSeverity)) +
				(penaltyValues.IWant * float64(p2p.HighErrSeverity)) +
				(penaltyValues.IWant * float64(p2p.ModerateErrSeverity)) +
				(penaltyValues.IWant * float64(p2p.LowErrSeverity)),
		},
	}

	for _, tCase := range testCases {
		// report a misbehavior for the peer id.
		reg.OnInvalidControlMessageNotification(tCase.notification)
		// the penalty should now be updated in the spamRecords
		record, err, ok := spamRecords.Get(tCase.notification.PeerID) // get the record from the spamRecords.
		assert.True(t, ok)
		assert.NoError(t, err)
		assert.Less(t, math.Abs(tCase.expectedPenalty-record.Penalty), 10e-3)  // penalty should be updated to -10.
		assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

		// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
		// and the peer should be deprived of the default reward for its valid staked role.
		score := reg.AppSpecificScoreFunc()(tCase.notification.PeerID)
		assert.Less(t, math.Abs(tCase.expectedPenalty-score), 10e-3)
	}
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

func withInitFunction(initFunction scoring.SpamRecordInitFunc) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Init = initFunction
	}
}

type scoringRegistryParamsOpt func(*scoring.GossipSubAppSpecificScoreRegistryConfig)

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry(t *testing.T, opts ...scoringRegistryParamsOpt) (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.GossipSubSpamRecordCache) {
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	scoringRegistryConfig := flowConfig.NetworkConfig.GossipSubConfig.GossipSubScoringRegistryConfig
	return newScoringRegistry(t, scoringRegistryConfig, opts...)
}

func newScoringRegistry(t *testing.T, config p2pconf.GossipSubScoringRegistryConfig, opts ...scoringRegistryParamsOpt) (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.GossipSubSpamRecordCache) {
	cache := netcache.NewGossipSubSpamRecordCache(
		100,
		unittest.Logger(),
		metrics.NewNoopCollector(),
		scoring.DefaultDecayFunction(config.PenaltyDecaySlowdownThreshold, config.DecayRateReductionFactor, config.PenaltyDecayEvaluationPeriod),
	)
	cfg := &scoring.GossipSubAppSpecificScoreRegistryConfig{
		Logger:     unittest.Logger(),
		Init:       scoring.InitAppScoreRecordState,
		Penalty:    penaltyValueFixtures(),
		IdProvider: mock.NewIdentityProvider(t),
		Validator:  mockp2p.NewSubscriptionValidator(t),
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
		Graft:                                 -100,
		Prune:                                 -50,
		IHave:                                 -20,
		IWant:                                 -10,
		ClusterPrefixedPenaltyReductionFactor: .5,
		RpcPublishMessage:                     -10,
	}
}

// penaltyValueFixture returns the set penalty of the provided control message type returned from the fixture func penaltyValueFixtures.
func penaltyValueFixture(msgType p2pmsg.ControlMessageType) float64 {
	penaltyValues := penaltyValueFixtures()
	switch msgType {
	case p2pmsg.CtrlMsgGraft:
		return penaltyValues.Graft
	case p2pmsg.CtrlMsgPrune:
		return penaltyValues.Prune
	case p2pmsg.CtrlMsgIHave:
		return penaltyValues.IHave
	case p2pmsg.CtrlMsgIWant:
		return penaltyValues.IWant
	case p2pmsg.RpcPublishMessage:
		return penaltyValues.RpcPublishMessage
	default:
		return penaltyValues.ClusterPrefixedPenaltyReductionFactor
	}
}

func randomErrSeverity() p2p.InvCtrlMsgErrSeverity {
	return []p2p.InvCtrlMsgErrSeverity{p2p.LowErrSeverity, p2p.ModerateErrSeverity, p2p.HighErrSeverity, p2p.CriticalErrSeverity}[rand.Intn(4)]
}
