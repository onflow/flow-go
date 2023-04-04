package scoring_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"

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

// TestGossipSubAppSpecificScoreRegistry_Get_Then_Report tests when a peer id is queried for the first time by the
// app specific score function, the score is initialized to the initial state. Then, the score is reported and the
// score is updated in the cache. The next time the app specific score function is called, the score should be the
// updated score.
func TestGossipSubAppSpecificScoreRegistry_Get_Then_Report(t *testing.T) {
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
		MsgType: p2p.CtrlMsgGraft,
		Count:   1,
	})

	// the score should now be updated.
	record, err, ok = cache.Get(peerID) // get the record from the cache.
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Less(t, math.Abs(scoring.DefaultGossipSubCtrlMsgPenaltyValue().Graft-record.Score), 10e-3) // score should be updated to -10.
	assert.Equal(t, scoring.InitAppScoreRecordState().Decay, record.Decay)                            // decay should be initialized to the initial state.

	// when the app specific score function is called again, the score should be updated.
	score = reg.AppSpecificScoreFunc()(peerID)
	assert.Less(t, math.Abs(scoring.DefaultGossipSubCtrlMsgPenaltyValue().Graft-score), 10e-3) // score should be updated to -10.
}

// TestGossipSubAppSpecificScoreRegistry_Report_Then_Get tests situation where a peer id is report for the first time
// before the app specific score function is called for the first time on it.
// The test expects the score to be initialized to the initial state and then updated by the penalty value.
// Subsequent calls to the app specific score function should return the updated score.
func TestGossipSubAppSpecificScoreRegistry_Report_Then_Get(t *testing.T) {
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

// TestGossipSubAppSpecificScoreRegistry_Concurrent_Report_And_Get tests concurrent calls to the app specific score
// and report function when there is no record in the cache about the peer.
// The test expects the score to be initialized to the initial state and then updated by the penalty value, regardless of
// the order of the calls.
func TestGossipSubAppSpecificScoreRegistry_Concurrent_Report_And_Get(t *testing.T) {
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

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry() (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.AppScoreCache) {
	cache := netcache.NewAppScoreCache(100, unittest.Logger(), metrics.NewNoopCollector(), scoring.DefaultDecayFunction())
	return scoring.NewGossipSubAppSpecificScoreRegistry(&scoring.GossipSubAppSpecificScoreRegistryConfig{
		SizeLimit:     100,
		Logger:        unittest.Logger(),
		Collector:     metrics.NewNoopCollector(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Penalty:       scoring.DefaultGossipSubCtrlMsgPenaltyValue(),
	}, scoring.WithScoreCache(cache)), cache
}
