package scoring_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/network/p2p/scoring"
)

func TestGeometricDecay(t *testing.T) {
	type args struct {
		score       float64
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
			name: "valid score, decay, and time",
			args: args{
				score:       100,
				decay:       0.9,
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want:    100 * math.Pow(0.9, 10),
			wantErr: nil,
		},
		{
			name: "zero decay factor",
			args: args{
				score:       100,
				decay:       0,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: nil,
		},
		{
			name: "decay factor of 1",
			args: args{
				score:       100,
				decay:       1,
				lastUpdated: time.Now().Add(-10 * time.Second),
			},
			want:    100,
			wantErr: nil,
		},
		{
			name: "negative decay factor",
			args: args{
				score:       100,
				decay:       -0.5,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: fmt.Errorf("decay factor must be in the range [0, 1], got %f", -0.5),
		},
		{
			name: "decay factor greater than 1",
			args: args{
				score:       100,
				decay:       1.2,
				lastUpdated: time.Now(),
			},
			want:    0,
			wantErr: fmt.Errorf("decay factor must be in the range [0, 1], got %f", 1.2),
		},
		{
			name: "large time value causing overflow",
			args: args{
				score:       100,
				decay:       0.999999999999999,
				lastUpdated: time.Now().Add(-1e5 * time.Second),
			},
			want:    100 * math.Pow(0.999999999999999, 1e5),
			wantErr: nil,
		},
		{
			name: "large decay factor and time value causing underflow",
			args: args{
				score:       100,
				decay:       0.999999,
				lastUpdated: time.Now().Add(-1e9 * time.Second),
			},
			want:    0,
			wantErr: nil,
		},
		{
			name: "very small decay factor and time value causing underflow",
			args: args{
				score:       100,
				decay:       0.000001,
				lastUpdated: time.Now().Add(-1e9 * time.Second),
			},
		},
		{
			name: "future time value causing an error",
			args: args{
				score:       100,
				decay:       0.999999,
				lastUpdated: time.Now().Add(+1e9 * time.Second),
			},
			want:    0,
			wantErr: fmt.Errorf("last updated time cannot be in the future"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scoring.GeometricDecay(tt.args.score, tt.args.decay, tt.args.lastUpdated)
			if tt.wantErr != nil {
				assert.Errorf(t, err, tt.wantErr.Error())
			}
			assert.Less(t, math.Abs(got-tt.want), 1e-3)
		})
	}
}
