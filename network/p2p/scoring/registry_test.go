package scoring_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p"
	netcache "github.com/onflow/flow-go/network/p2p/cache"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestScoreRegistry_FreshStart tests the app specific score computation of the node when there is no spam record for the peer id upon fresh start of the registry.
// It tests the state that a staked peer with a valid role and valid subscriptions has no spam records; hence it should "eventually" be rewarded with the default reward
// for its GossipSub app specific score. The "eventually" comes from the fact that the app specific score is updated asynchronously in the cache, and the cache is
// updated when the app specific score function is called by GossipSub.
func TestScoreRegistry_FreshStart(t *testing.T) {
	peerID := peer.ID("peer-1")

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id, and there should be no app-specific score in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	queryTime := time.Now()
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		return score == scoring.MaxAppSpecificReward
	}, 5*time.Second, 100*time.Millisecond)

	// still the spamRecords should not have the peer id (as there is no spam record for the peer id).
	require.False(t, spamRecords.Has(peerID))

	// however, the app specific score should be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))
	require.Equal(t, scoring.MaxAppSpecificReward, score)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_PeerWithSpamRecord is a test suite designed to assess the app-specific penalty computation
// in a scenario where a peer with a staked identity and valid subscriptions has a spam record. The suite runs multiple
// sub-tests, each targeting a specific type of control message (graft, prune, ihave, iwant, RpcPublishMessage). The focus
// is on the impact of spam records on the app-specific score, specifically how such records negate the default reward
// a staked peer would otherwise receive, leaving only the penalty as the app-specific score. This testing reflects the
// asynchronous nature of app-specific score updates in GossipSub's cache.
func TestScoreRegistry_PeerWithSpamRecord(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistryPeerWithSpamRecord(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

// testScoreRegistryPeerWithSpamRecord conducts an individual test within the TestScoreRegistry_PeerWithSpamRecord suite.
// It evaluates the ScoreRegistry's handling of a staked peer with valid subscriptions when a spam record is present for
// the peer ID. The function simulates the process of starting the registry, recording a misbehavior, and then verifying the
// updates to the spam records and app-specific score cache based on the type of control message received.
// Parameters:
// - t *testing.T: The test context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// This function specifically tests how the ScoreRegistry updates a peer's app-specific score in response to spam records,
// emphasizing the removal of the default reward for staked peers with valid roles and focusing on the asynchronous update
// mechanism of the app-specific score in the cache.
func testScoreRegistryPeerWithSpamRecord(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")

	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// since the peer id does not have a spam record, the app specific score should be the max app specific reward, which
		// is the default reward for a staked peer that has valid subscriptions.
		return scoring.MaxAppSpecificReward == score
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
	})

	// the penalty should now be updated in the spamRecords
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)        // penalty should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// this peer has a spam record, with no subscription penalty. Hence, the app specific score should only be the spam penalty,
		// and the peer should be deprived of the default reward for its valid staked role.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		return math.Abs(expectedPenalty-score)/math.Max(expectedPenalty, score) < 0.001
	}, 5*time.Second, 100*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))
	require.True(t, math.Abs(expectedPenalty-score)/math.Max(expectedPenalty, score) < 0.001)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_SpamRecordWithUnknownIdentity is a test suite for verifying the behavior of the ScoreRegistry
// when handling spam records associated with unknown identities. It tests various scenarios based on different control
// message types, including graft, prune, ihave, iwant, and RpcPublishMessage. Each sub-test validates the app-specific
// penalty computation and updates to the score registry when a peer with an unknown identity sends these control messages.
func TestScoreRegistry_SpamRecordWithUnknownIdentity(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistrySpamRecordWithUnknownIdentity(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

// testScoreRegistrySpamRecordWithUnknownIdentity tests the app-specific penalty computation of the node when there
// is a spam record for a peer ID with an unknown identity. It examines the functionality of the GossipSubAppSpecificScoreRegistry
// under various conditions, including the initialization state, spam record creation, and the impact of different control message types.
// Parameters:
// - t *testing.T: The testing context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// The function simulates the process of starting the registry, reporting a misbehavior for the peer ID, and verifying the
// updates to the spam records and app-specific score cache. It ensures that the penalties are correctly computed and applied
// based on the given control message type and the state of the peer ID (unknown identity and spam record presence).
func testScoreRegistrySpamRecordWithUnknownIdentity(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(
		t,
		withUnknownIdentity(peerID),
		withValidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	// eventually the app specific score should be updated in the cache to the penalty value for unknown identity.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// peer does not have spam record, but has an unknown identity. Hence, the app specific score should be the staking penalty.
		return scoring.DefaultUnknownIdentityPenalty == score
	}, 5*time.Second, 100*time.Millisecond)

	// queryTime := time.Now()
	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
	})

	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	require.True(t, ok)
	require.NoError(t, err)
	require.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)        // penalty should be updated to -10, we account for decay.
	require.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
		// and the staking penalty.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		nominator := math.Abs(expectedPenalty + scoring.DefaultUnknownIdentityPenalty - score)
		denominator := math.Max(expectedPenalty+scoring.DefaultUnknownIdentityPenalty, score)
		return math.Abs(nominator/denominator) < 0.001
	}, 5*time.Second, 100*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))

	nominator := math.Abs(expectedPenalty + scoring.DefaultUnknownIdentityPenalty - score)
	denominator := math.Max(expectedPenalty+scoring.DefaultUnknownIdentityPenalty, score)
	require.True(t, math.Abs(nominator/denominator) < 0.001)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestScoreRegistry_SpamRecordWithSubscriptionPenalty is a test suite for verifying the behavior of the ScoreRegistry
// in handling spam records associated with invalid subscriptions. It encompasses a series of sub-tests, each focusing on
// a different control message type: graft, prune, ihave, iwant, and RpcPublishMessage. These sub-tests are designed to
// validate the appropriate application of penalties in the ScoreRegistry when a peer with an invalid subscription is involved
// in spam activities, as indicated by these control messages.
func TestScoreRegistry_SpamRecordWithSubscriptionPenalty(t *testing.T) {
	t.Run("graft", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgGraft, penaltyValueFixtures().Graft)
	})
	t.Run("prune", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgPrune, penaltyValueFixtures().Prune)
	})
	t.Run("ihave", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIHave, penaltyValueFixtures().IHave)
	})
	t.Run("iwant", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.CtrlMsgIWant, penaltyValueFixtures().IWant)
	})
	t.Run("RpcPublishMessage", func(t *testing.T) {
		testScoreRegistrySpamRecordWithSubscriptionPenalty(t, p2pmsg.RpcPublishMessage, penaltyValueFixtures().RpcPublishMessage)
	})
}

// testScoreRegistrySpamRecordWithSubscriptionPenalty tests the application-specific penalty computation in the ScoreRegistry
// when a spam record exists for a peer ID that also has an invalid subscription. The function simulates the process of
// initializing the registry, handling spam records, and updating penalties based on various control message types.
// Parameters:
// - t *testing.T: The testing context.
// - messageType p2pmsg.ControlMessageType: The type of control message being tested.
// - expectedPenalty float64: The expected penalty value for the given control message type.
// The function focuses on evaluating the registry's response to spam activities (as represented by control messages) from a
// peer with invalid subscriptions. It verifies that penalties are accurately computed and applied, taking into account both
// the spam record and the invalid subscription status of the peer.
func testScoreRegistrySpamRecordWithSubscriptionPenalty(t *testing.T, messageType p2pmsg.ControlMessageType, expectedPenalty float64) {
	peerID := peer.ID("peer-1")
	reg, spamRecords, appScoreCache := newGossipSubAppSpecificScoreRegistry(
		t,
		withStakedIdentity(peerID),
		withInvalidSubscriptions(peerID))

	// starts the registry.
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	reg.Start(signalerCtx)
	unittest.RequireCloseBefore(t, reg.Ready(), 1*time.Second, "failed to start GossipSubAppSpecificScoreRegistry")

	// initially, the spamRecords should not have the peer id; also the app specific score record should not be in the cache.
	require.False(t, spamRecords.Has(peerID))
	score, updated, exists := appScoreCache.Get(peerID) // get the score from the cache.
	require.False(t, exists)
	require.Equal(t, time.Time{}, updated)
	require.Equal(t, float64(0), score)

	// peer does not have spam record, but has invalid subscription. Hence, the app specific score should be subscription penalty.
	// eventually the app specific score should be updated in the cache to the penalty value for subscription penalty.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// peer does not have spam record, but has an invalid subscription penalty.
		return scoring.DefaultInvalidSubscriptionPenalty == score
	}, 5*time.Second, 100*time.Millisecond)

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: messageType,
	})

	// the penalty should now be updated.
	record, err, ok := spamRecords.Get(peerID) // get the record from the spamRecords.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Penalty), 10e-3)
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	queryTime := time.Now()
	// eventually, the app specific score should be updated in the cache.
	require.Eventually(t, func() bool {
		// calling the app specific score function when there is no app specific score in the cache should eventually update the cache.
		score := reg.AppSpecificScoreFunc()(peerID)
		// the peer has spam record as well as an unknown identity. Hence, the app specific score should be the spam penalty
		// and the staking penalty.
		// As the app specific score in the cache and spam penalty in the spamRecords are updated at different times, we account for 0.1% error.
		nominator := math.Abs(expectedPenalty + scoring.DefaultInvalidSubscriptionPenalty - score)
		denominator := math.Max(expectedPenalty+scoring.DefaultInvalidSubscriptionPenalty, score)
		return math.Abs(nominator/denominator) < 0.001
	}, 5*time.Second, 100*time.Millisecond)

	// the app specific score should now be updated in the cache.
	score, updated, exists = appScoreCache.Get(peerID) // get the score from the cache.
	require.True(t, exists)
	require.True(t, updated.After(queryTime))

	nominator := math.Abs(expectedPenalty + scoring.DefaultInvalidSubscriptionPenalty - score)
	denominator := math.Max(expectedPenalty+scoring.DefaultInvalidSubscriptionPenalty, score)
	require.True(t, math.Abs(nominator/denominator) < 0.001)

	// stop the registry.
	cancel()
	unittest.RequireCloseBefore(t, reg.Done(), 1*time.Second, "failed to stop GossipSubAppSpecificScoreRegistry")
}

// TestSpamPenaltyDecaysInCache tests that the spam penalty records decay over time in the cache.
func TestSpamPenaltyDecaysInCache(t *testing.T) {
	peerID := peer.ID("peer-1")
	reg, _, _ := newGossipSubAppSpecificScoreRegistry(t,
		withStakedIdentity(peerID),
		withValidSubscriptions(peerID))

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgPrune,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIHave,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgIWant,
	})

	time.Sleep(1 * time.Second) // wait for the penalty to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.RpcPublishMessage,
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
		penaltyValueFixtures().IWant +
		penaltyValueFixtures().RpcPublishMessage
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
	peerID := peer.ID("peer-1")
	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(
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
	peerID := peer.ID("peer-1")
	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(
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
	peerID := peer.ID("peer-1")
	reg, spamRecords, _ := newGossipSubAppSpecificScoreRegistry(
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

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
		PeerID:  peerID,
		MsgType: p2pmsg.CtrlMsgGraft,
	})

	// with reported spam, the app specific score should be the default invalid subscription penalty + the spam penalty.
	require.Less(t, math.Abs(scoring.DefaultInvalidSubscriptionPenalty+penaltyValueFixtures().Graft-reg.AppSpecificScoreFunc()(peerID)), 10e-3)

	// decays happen every second, so we wait for 1 second to make sure the penalty is updated.
	time.Sleep(1 * time.Second)
	// the penalty should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                                                                // the penalty should be less than zero.
	require.Greater(t, score, penaltyValueFixtures().Graft+scoring.DefaultInvalidSubscriptionPenalty) // the penalty should be less than the penalty value due to decay.

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
		cfg.Validator.(*mockp2p.SubscriptionValidator).On("CheckSubscribedToAllowedTopics",
			peer,
			testifymock.Anything).Return(fmt.Errorf("invalid subscriptions")).Maybe()
	}
}

func withInitFunction(initFunction func() p2p.GossipSubSpamRecord) func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
	return func(cfg *scoring.GossipSubAppSpecificScoreRegistryConfig) {
		cfg.Init = initFunction
	}
}

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry(t *testing.T, opts ...func(*scoring.GossipSubAppSpecificScoreRegistryConfig)) (*scoring.GossipSubAppSpecificScoreRegistry,
	*netcache.GossipSubSpamRecordCache,
	*internal.AppSpecificScoreCache) {
	cache := netcache.NewGossipSubSpamRecordCache(100, unittest.Logger(), metrics.NewNoopHeroCacheMetricsFactory(), scoring.DefaultDecayFunction())
	appSpecificScoreCache := internal.NewAppSpecificScoreCache(100, unittest.Logger(), metrics.NewNoopHeroCacheMetricsFactory())
	flowCfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// overrides the default values for testing purposes.
	flowCfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore.ScoreTTL = 100 * time.Millisecond

	validator := mockp2p.NewSubscriptionValidator(t)
	validator.On("Start", testifymock.Anything).Return().Maybe()
	done := make(chan struct{})
	close(done)
	f := func() <-chan struct{} {
		return done
	}
	validator.On("Ready").Return(f()).Maybe()
	validator.On("Done").Return(f()).Maybe()
	cfg := &scoring.GossipSubAppSpecificScoreRegistryConfig{
		Logger:     unittest.Logger(),
		Init:       scoring.InitAppScoreRecordState,
		Penalty:    penaltyValueFixtures(),
		IdProvider: mock.NewIdentityProvider(t),
		Validator:  validator,
		AppScoreCacheFactory: func() p2p.GossipSubApplicationSpecificScoreCache {
			return appSpecificScoreCache
		},
		SpamRecordCacheFactory: func() p2p.GossipSubSpamRecordCache {
			return cache
		},
		Parameters:              flowCfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore,
		HeroCacheMetricsFactory: metrics.NewNoopHeroCacheMetricsFactory(),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	reg, err := scoring.NewGossipSubAppSpecificScoreRegistry(cfg)
	require.NoError(t, err, "failed to create GossipSubAppSpecificScoreRegistry")

	return reg, cache, appSpecificScoreCache
}

// penaltyValueFixtures returns a set of penalty values for testing purposes.
// The values are not realistic. The important thing is that they are different from each other. This is to make sure
// that the tests are not passing because of the default values.
func penaltyValueFixtures() scoring.GossipSubCtrlMsgPenaltyValue {
	return scoring.GossipSubCtrlMsgPenaltyValue{
		Graft:             -100,
		Prune:             -50,
		IHave:             -20,
		IWant:             -10,
		RpcPublishMessage: -10,
	}
}
