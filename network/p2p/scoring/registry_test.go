package scoring_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p/scoring"
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
