package scoring_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
)

// TestGeometricDecay tests the GeometricDecay function.
func TestGeometricDecay(t *testing.T) {
	type args struct {
		penalty     float64
		decay       float64
		lastUpdated time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr error
	}{
		{
			name: "valid penalty, decay, and time",
			args: args{
				penalty:     100,
				decay:       0.9,
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want:    100 * math.Pow(0.9, 10),
			wantErr: nil,
		},
		{
			name: "zero decay factor",
			args: args{
				penalty:     100,
				decay:       0,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: fmt.Errorf("decay factor must be in the range [0, 1], got 0"),
		},
		{
			name: "decay factor of 1",
			args: args{
				penalty:     100,
				decay:       1,
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want:    100,
			wantErr: nil,
		},
		{
			name: "negative decay factor",
			args: args{
				penalty:     100,
				decay:       -0.5,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: fmt.Errorf("decay factor must be in the range [0, 1], got %f", -0.5),
		},
		{
			name: "decay factor greater than 1",
			args: args{
				penalty:     100,
				decay:       1.2,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: fmt.Errorf("decay factor must be in the range [0, 1], got %f", 1.2),
		},
		{
			name: "large time value causing overflow",
			args: args{
				penalty:     100,
				decay:       0.999999999999999,
				lastUpdated: time.Now().Add(-1e5 * time.Second),
			},
			want:    100 * math.Pow(0.999999999999999, 1e5),
			wantErr: nil,
		},
		{
			name: "large decay factor and time value causing underflow",
			args: args{
				penalty:     100,
				decay:       0.999999,
				lastUpdated: time.Now().Add(-1e9 * time.Second),
			},
			want:    0,
			wantErr: nil,
		},
		{
			name: "very small decay factor and time value causing underflow",
			args: args{
				penalty:     100,
				decay:       0.000001,
				lastUpdated: time.Now().Add(-1e9 * time.Second),
			},
			want:    0,
			wantErr: nil,
		},
		{
			name: "future time value causing an error",
			args: args{
				penalty:     100,
				decay:       0.999999,
				lastUpdated: time.Now().Add(+1e9 * time.Second),
			},
			want:    0,
			wantErr: fmt.Errorf("last updated time cannot be in the future"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scoring.GeometricDecay(tt.args.penalty, tt.args.decay, tt.args.lastUpdated)
			if tt.wantErr != nil {
				assert.Errorf(t, err, tt.wantErr.Error())
			}
			assert.Less(t, math.Abs(got-tt.want), 1e-3)
		})
	}
}

// TestDefaultDecayFunction tests the default decay function.
// The default decay function is used when no custom decay function is provided.
// The test evaluates the following cases:
// 1. penalty is non-negative and should not be decayed.
// 2. penalty is negative and above the skipDecayThreshold and lastUpdated is too recent. In this case, the penalty should not be decayed.
// 3. penalty is negative and above the skipDecayThreshold and lastUpdated is too old. In this case, the penalty should not be decayed.
// 4. penalty is negative and below the skipDecayThreshold and lastUpdated is too recent. In this case, the penalty should not be decayed.
// 5. penalty is negative and below the skipDecayThreshold and lastUpdated is too old. In this case, the penalty should be decayed.
func TestDefaultDecayFunction(t *testing.T) {
	type args struct {
		record      p2p.GossipSubSpamRecord
		lastUpdated time.Time
	}

	type want struct {
		record p2p.GossipSubSpamRecord
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			// 1. penalty is non-negative and should not be decayed.
			name: "penalty is non-negative",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: 5,
					Decay:   0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: 5,
					Decay:   0.8,
				},
			},
		},
		{
			// 2. penalty is negative and above the skipDecayThreshold and lastUpdated is too recent. In this case, the penalty should not be decayed,
			// since less than a second has passed since last update.
			name: "penalty is negative and but above skipDecayThreshold and lastUpdated is too recent",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: -0.09, // -0.09 is above skipDecayThreshold of -0.1
					Decay:   0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: 0, // penalty is set to 0
					Decay:   0.8,
				},
			},
		},
		{
			// 3. penalty is negative and above the skipDecayThreshold and lastUpdated is too old. In this case, the penalty should not be decayed,
			// since penalty is between [skipDecayThreshold, 0] and more than a second has passed since last update.
			name: "penalty is negative and but above skipDecayThreshold and lastUpdated is too old",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: -0.09, // -0.09 is above skipDecayThreshold of -0.1
					Decay:   0.8,
				},
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: 0, // penalty is set to 0
					Decay:   0.8,
				},
			},
		},
		{
			// 4. penalty is negative and below the skipDecayThreshold and lastUpdated is too recent. In this case, the penalty should not be decayed,
			// since less than a second has passed since last update.
			name: "penalty is negative and below skipDecayThreshold but lastUpdated is too recent",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: -5,
					Decay:   0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: -5,
					Decay:   0.8,
				},
			},
		},
		{
			// 5. penalty is negative and below the skipDecayThreshold and lastUpdated is too old. In this case, the penalty should be decayed.
			name: "penalty is negative and below skipDecayThreshold but lastUpdated is too old",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: -15,
					Decay:   0.8,
				},
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: -15 * math.Pow(0.8, 10),
					Decay:   0.8,
				},
			},
		},
	}

	decayFunc := scoring.DefaultDecayFunction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decayFunc(tt.args.record, tt.args.lastUpdated)
			assert.NoError(t, err)
			assert.Less(t, math.Abs(got.Penalty-tt.want.record.Penalty), 10e-3)
			assert.Equal(t, got.Decay, tt.want.record.Decay)
		})
	}
}
