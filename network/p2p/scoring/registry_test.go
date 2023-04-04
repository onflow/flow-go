package scoring_test

import (
	"math"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/metrics"
	netcache "github.com/onflow/flow-go/network/cache"
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

// newGossipSubAppSpecificScoreRegistry returns a new instance of GossipSubAppSpecificScoreRegistry with default values
// for the testing purposes.
func newGossipSubAppSpecificScoreRegistry() (*scoring.GossipSubAppSpecificScoreRegistry, *netcache.AppScoreCache) {
	cache := netcache.NewAppScoreCache(100, unittest.Logger(), metrics.NewNoopCollector())
	return scoring.NewGossipSubAppSpecificScoreRegistry(&scoring.GossipSubAppSpecificScoreRegistryConfig{
		SizeLimit:     100,
		Logger:        unittest.Logger(),
		Collector:     metrics.NewNoopCollector(),
		DecayFunction: scoring.DefaultDecayFunction(),
		Penalty:       scoring.DefaultGossipSubCtrlMsgPenaltyValue(),
	}, scoring.WithScoreCache(cache)), cache
}
