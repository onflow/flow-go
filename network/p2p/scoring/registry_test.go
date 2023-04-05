package scoring_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDefaultDecayFunction tests the default decay function used by the peer scorer.
// The default decay function is used when no custom decay function is provided.
// The test evaluates the following cases:
// 1. score is non-negative and should not be decayed.
// 2. score is negative and above the skipDecayThreshold and lastUpdated is too recent. In this case, the score should not be decayed.
// 3. score is negative and above the skipDecayThreshold and lastUpdated is too old. In this case, the score should not be decayed.
// 4. score is negative and below the skipDecayThreshold and lastUpdated is too recent. In this case, the score should not be decayed.
// 5. score is negative and below the skipDecayThreshold and lastUpdated is too old. In this case, the score should be decayed.
func TestDefaultDecayFunction(t *testing.T) {
	type args struct {
		record      netcache.AppScoreRecord
		lastUpdated time.Time
	}

	type want struct {
		record netcache.AppScoreRecord
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			// 1. score is non-negative and should not be decayed.
			name: "score is non-negative",
			args: args{
				record: netcache.AppScoreRecord{
					Score: 5,
					Decay: 0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: netcache.AppScoreRecord{
					Score: 5,
					Decay: 0.8,
				},
			},
		},
		{ // 2. score is negative and above the skipDecayThreshold and lastUpdated is too recent. In this case, the score should not be decayed.
			name: "score is negative and but above skipDecayThreshold and lastUpdated is too recent",
			args: args{
				record: netcache.AppScoreRecord{
					Score: -0.09, // -0.09 is above skipDecayThreshold of -0.1
					Decay: 0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: netcache.AppScoreRecord{
					Score: 0, // score is set to 0
					Decay: 0.8,
				},
			},
		},
		{
			// 3. score is negative and above the skipDecayThreshold and lastUpdated is too old. In this case, the score should not be decayed.
			name: "score is negative and but above skipDecayThreshold and lastUpdated is too old",
			args: args{
				record: netcache.AppScoreRecord{
					Score: -0.09, // -0.09 is above skipDecayThreshold of -0.1
					Decay: 0.8,
				},
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want: want{
				record: netcache.AppScoreRecord{
					Score: 0, // score is set to 0
					Decay: 0.8,
				},
			},
		},
		{
			// 4. score is negative and below the skipDecayThreshold and lastUpdated is too recent. In this case, the score should not be decayed.
			name: "score is negative and below skipDecayThreshold but lastUpdated is too recent",
			args: args{
				record: netcache.AppScoreRecord{
					Score: -5,
					Decay: 0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: netcache.AppScoreRecord{
					Score: -5,
					Decay: 0.8,
				},
			},
		},
		{
			// 5. score is negative and below the skipDecayThreshold and lastUpdated is too old. In this case, the score should be decayed.
			name: "score is negative and below skipDecayThreshold but lastUpdated is too old",
			args: args{
				record: netcache.AppScoreRecord{
					Score: -15,
					Decay: 0.8,
				},
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want: want{
				record: netcache.AppScoreRecord{
					Score: -15 * math.Pow(0.8, 10),
					Decay: 0.8,
				},
			},
		},
	}

	decayFunc := scoring.DefaultDecayFunction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decayFunc(tt.args.record, tt.args.lastUpdated)
			assert.NoError(t, err)
			assert.Less(t, math.Abs(got.Score-tt.want.record.Score), 10e-3)
			assert.Equal(t, got.Decay, tt.want.record.Decay)
		})
	}
}

// TestGossipSubAppSpecificScoreRegistry_AppSpecificScoreFunc_Init tests when a peer id is queried for the first time by the
// app specific score function, the score is initialized to the initial state.
func TestGossipSubAppSpecificScoreRegistry_AppSpecificScoreFunc_Init(t *testing.T) {
	reg, cache := newGossipSubAppSpecificScoreRegistry()
	peerID := peer.ID("peer-1")

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// when the app specific score function is called for the first time, the score should be initialized to the initial state.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, score, scoring.InitAppScoreRecordState().Score) // score should be initialized to the initial state.

	// the cache should now have the peer id.
	assert.True(t, cache.Has(peerID))
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, record.Score, scoring.InitAppScoreRecordState().Score) // score should be initialized to the initial state.
	assert.Equal(t, record.Decay, scoring.InitAppScoreRecordState().Decay) // decay should be initialized to the initial state.
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

// testInitWhenGetFirst tests when a peer id is queried for the first time by the
// app specific score function, the score is initialized to the initial state. Then, the score is reported and the
// score is updated in the cache. The next time the app specific score function is called, the score should be the
// updated score.
func testInitWhenGetFirst(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	reg, cache := newGossipSubAppSpecificScoreRegistry()
	peerID := peer.ID("peer-1")

	// initially, the cache should not have the peer id.
	assert.False(t, cache.Has(peerID))

	// when the app specific score function is called for the first time, the score should be initialized to the initial state.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Equal(t, score, scoring.InitAppScoreRecordState().Score) // score should be initialized to the initial state.

	// the cache should now have the peer id.
	assert.True(t, cache.Has(peerID))
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, record.Score, scoring.InitAppScoreRecordState().Score) // score should be initialized to the initial state.
	assert.Equal(t, record.Decay, scoring.InitAppScoreRecordState().Decay) // decay should be initialized to the initial state.

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: messageType,
		Count:   1,
	})

	// the score should now be updated.
	record, err, ok = cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(expectedPenalty-record.Score), 10e-3)          // score should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay) // decay should be initialized to the initial state.

	// when the app specific score function is called again, the score should be updated.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(expectedPenalty-score), 10e-3) // score should be updated to -10.
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

// testInitWhenReportGoesFirst tests situation where a peer id is reported for the first time
// before the app specific score function is called for the first time on it.
// The test expects the score to be initialized to the initial state and then updated by the penalty value.
// Subsequent calls to the app specific score function should return the updated score.
func testInitWhenReportGoesFirst(t *testing.T, messageType p2p.ControlMessageType, expectedPenalty float64) {
	reg, cache := newGossipSubAppSpecificScoreRegistry()
	peerID := peer.ID("peer-1")

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	// the score should now be updated.
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(scoring.DefaultGossipSubCtrlMsgPenaltyValue().Graft-record.Score), 10e-3) // score should be updated to -10, we account for decay.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay)                            // decay should be initialized to the initial state.

	// when the app specific score function is called for the first time, the score should be updated.
	score := reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(scoring.DefaultGossipSubCtrlMsgPenaltyValue().Graft-score), 10e-3) // score should be updated to -10, we account for decay.
}

// TestScoreDecays tests that the score decays over time.
func TestScoreDecays(t *testing.T) {
	reg, _ := newGossipSubAppSpecificScoreRegistry()
	peerID := peer.ID("peer-1")

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgPrune,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the score to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the score to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgIHave,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the score to decay.

	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgIWant,
		Count:   1,
	})

	time.Sleep(1 * time.Second) // wait for the score to decay.

	// when the app specific score function is called for the first time, the score should be updated.
	score := reg.AppSpecificScoreFunc()(peerID)
	// the upper bound is the sum of the penalties without decay.
	scoreUpperBound := penaltyValueFixtures().Prune +
		penaltyValueFixtures().Graft +
		penaltyValueFixtures().IHave +
		penaltyValueFixtures().IWant
	// the lower bound is the sum of the penalties with decay assuming the decay is applied 4 times to the sum of the penalties.
	// in reality, the decay is applied 4 times to the first penalty, then 3 times to the second penalty, and so on.
	scoreLowerBound := scoreUpperBound * math.Pow(scoring.InitAppScoreRecordState().Decay, 4)

	// with decay, the score should be between the upper and lower bounds.
	assert.Greater(t, score, scoreUpperBound)
	assert.Less(t, score, scoreLowerBound)
}

// TestConcurrentGetAndReport tests concurrent calls to the app specific score
// and report function when there is no record in the cache about the peer.
// The test expects the score to be initialized to the initial state and then updated by the penalty value, regardless of
// the order of the calls.
func TestConcurrentGetAndReport(t *testing.T) {
	reg, cache := newGossipSubAppSpecificScoreRegistry()
	peerID := peer.ID("peer-1")

	wg := sync.WaitGroup{} // wait group to wait for all the go routines to finish.
	wg.Add(2)              // we expect 2 go routines to finish.

	// go routine to call the app specific score function.
	go func() {
		defer wg.Done()
		score := reg.AppSpecificScoreFunc()(peerID)
		assert.Equal(t, score, scoring.InitAppScoreRecordState().Score) // score should be initialized to the initial state.
	}()

	// go routine to report a misbehavior for the peer id.
	go func() {
		defer wg.Done()
		reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
			PeerID:  peerID,
			MsgType: p2p.CtrlMsgGraft,
			Count:   1,
		})
	}()

	unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "goroutines are not done on time") // wait for the go routines to finish.

	// the score should now be updated.
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(scoring.DefaultGossipSubCtrlMsgPenaltyValue().Graft-record.Score), 10e-3) // score should be updated to -10.
}

// TestDecayToZero tests that the score decays to zero. The test expects the score to be updated to the penalty value
// and then decay to zero over time.
func TestDecayToZero(t *testing.T) {
	cache := netcache.NewAppScoreCache(100, unittest.Logger(), metrics.NewNoopCollector(), scoring.DefaultDecayFunction())
	reg := scoring.NewGossipSubAppSpecificScoreRegistry(&scoring.GossipSubAppSpecificScoreRegistryConfig{
		SizeLimit:     100,
		Logger:        unittest.Logger(),
		Collector:     metrics.NewNoopCollector(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Penalty:       penaltyValueFixtures(),
	}, scoring.WithScoreCache(cache), scoring.WithRecordInit(func() netcache.AppScoreRecord {
		return netcache.AppScoreRecord{
			Decay: 0.02, // we choose a small decay value to speed up the test.
			Score: 0,
		}
	}))

	peerID := peer.ID("peer-1")

	// report a misbehavior for the peer id.
	reg.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
		PeerID:  peerID,
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	// decays happen every second, so we wait for 1 second to make sure the score is updated.
	time.Sleep(1 * time.Second)
	// the score should now be updated, it should be still negative but greater than the penalty value (due to decay).
	score := reg.AppSpecificScoreFunc()(peerID)
	require.Less(t, score, float64(0))                      // the score should be less than zero.
	require.Greater(t, score, penaltyValueFixtures().Graft) // the score should be less than the penalty value due to decay.

	// wait for the score to decay to zero.
	require.Eventually(t, func() bool {
		score := reg.AppSpecificScoreFunc()(peerID)
		return score == 0 // the score should eventually decay to zero.
	}, 5*time.Second, 100*time.Millisecond)

	// the score should now be zero.
	record, err, ok := cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, record.Score) // score should be zero.
}

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry() (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.AppScoreCache) {
	cache := netcache.NewAppScoreCache(100, unittest.Logger(), metrics.NewNoopCollector(), scoring.DefaultDecayFunction())
	return scoring.NewGossipSubAppSpecificScoreRegistry(&scoring.GossipSubAppSpecificScoreRegistryConfig{
		SizeLimit:     100,
		Logger:        unittest.Logger(),
		Collector:     metrics.NewNoopCollector(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Penalty:       penaltyValueFixtures(),
	}, scoring.WithScoreCache(cache)), cache
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
