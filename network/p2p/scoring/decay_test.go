package scoring_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/config"
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
			assert.LessOrEqual(t, truncateFloat(math.Abs(got-tt.want), 3), 1e-2)
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
	flowConfig, err := config.DefaultConfig()
	assert.NoError(t, err)

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
					Penalty:             0, // penalty is set to 0
					Decay:               0.8,
					LastDecayAdjustment: time.Time{},
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
					Penalty:             0, // penalty is set to 0
					Decay:               0.8,
					LastDecayAdjustment: time.Time{},
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
		{
			// 6. penalty is negative and below slowerDecayPenaltyThreshold record decay should be adjusted. The `LastDecayAdjustment` has not been updated since initialization.
			name: "penalty is negative and below slowerDecayPenaltyThreshold record decay should be adjusted",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty: -100,
					Decay:   0.8,
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: -100,
					Decay:   0.81,
				},
			},
		},
		{
			// 7. penalty is negative and below slowerDecayPenaltyThreshold but record.LastDecayAdjustment is too recent. In this case the decay should not be adjusted.
			name: "penalty is negative and below slowerDecayPenaltyThreshold record decay should not be adjusted",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty:             -100,
					Decay:               0.9,
					LastDecayAdjustment: time.Now().Add(10 * time.Second),
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: -100,
					Decay:   0.9,
				},
			},
		},
		{
			// 8. penalty is negative and below slowerDecayPenaltyThreshold; and LastDecayAdjustment time passed the decay adjust interval. record decay should be adjusted.
			name: "penalty is negative and below slowerDecayPenaltyThreshold and LastDecayAdjustment time passed the decay adjust interval. Record decay should be adjusted",
			args: args{
				record: p2p.GossipSubSpamRecord{
					Penalty:             -100,
					Decay:               0.8,
					LastDecayAdjustment: time.Now().Add(-flowConfig.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.PenaltyDecayEvaluationPeriod),
				},
				lastUpdated: time.Now(),
			},
			want: want{
				record: p2p.GossipSubSpamRecord{
					Penalty: -100,
					Decay:   0.81,
				},
			},
		},
	}
	scoringRegistryConfig := flowConfig.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters
	decayFunc := scoring.DefaultDecayFunction(&scoring.DecayFunctionConfig{
		SlowerDecayPenaltyThreshold:   scoringRegistryConfig.SpamRecordCache.Decay.PenaltyDecaySlowdownThreshold,
		DecayRateReductionFactor:      scoringRegistryConfig.SpamRecordCache.Decay.DecayRateReductionFactor,
		DecayAdjustInterval:           scoringRegistryConfig.SpamRecordCache.Decay.PenaltyDecayEvaluationPeriod,
		MaximumSpamPenaltyDecayFactor: flowConfig.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MaximumSpamPenaltyDecayFactor,
		MinimumSpamPenaltyDecayFactor: flowConfig.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.MinimumSpamPenaltyDecayFactor,
		SkipDecayThreshold:            flowConfig.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.SpamRecordCache.Decay.SkipDecayThreshold,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decayFunc(tt.args.record, tt.args.lastUpdated)
			assert.NoError(t, err)
			tolerance := 0.01 // 1% tolerance
			expectedPenalty := tt.want.record.Penalty

			// ensure expectedPenalty is not zero to avoid division by zero
			if expectedPenalty != 0 {
				normalizedDifference := math.Abs(got.Penalty-expectedPenalty) / math.Abs(expectedPenalty)
				assert.Less(t, normalizedDifference, tolerance)
			} else {
				// handles the case where expectedPenalty is zero
				assert.Less(t, math.Abs(got.Penalty), tolerance)
			}
			assert.Equal(t, tt.want.record.Decay, got.Decay)
		})
	}
}

func truncateFloat(number float64, decimalPlaces int) float64 {
	pow := math.Pow(10, float64(decimalPlaces))
	return float64(int(number*pow)) / pow
}
