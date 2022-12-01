package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_computeTPS(t *testing.T) {
	now := time.Now()

	type args struct {
		lastTxs     uint
		currentTxs  uint
		lastTs      time.Time
		nowTs       time.Time
		lastTPS     float64
		targetTPS   uint
		inflight    int
		maxInflight uint
		timedout    bool
	}
	tests := []struct {
		name             string
		args             args
		wantSkip         bool
		wantCurrentTPS   float64
		wantUnboundedTPS uint
	}{
		{
			"empty",
			args{},
			true, 0, 0,
		},
		{
			"current TPS is equal or higher than target and inflight is higher than max",
			args{lastTxs: 1, currentTxs: 11, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 10, inflight: 1000, maxInflight: 100},
			false, 10, 10 + additiveIncrease,
		},
		{
			"current TPS is equal or higher than target and inflight is higher than max and timeout happened",
			args{lastTxs: 1, currentTxs: 11, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 10, inflight: 1000, maxInflight: 100, timedout: true},
			false, 10, 10 * multiplicativeDecrease,
		},
		{
			"current TPS is within the target and inflight is higher than max",
			args{lastTxs: 1, currentTxs: 10, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 10, inflight: 1000, maxInflight: 100},
			false, 9, 10 * multiplicativeDecrease,
		},
		{
			"current TPS is within the target and inflight is lower than max",
			args{lastTxs: 1, currentTxs: 10, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 10, inflight: 10, maxInflight: 100},
			false, 9, 10 + additiveIncrease,
		},
		{
			"current TPS is lower than target TPS but increasing and within max inflight",
			args{lastTxs: 1, currentTxs: 10, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 100, inflight: 10, maxInflight: 100},
			true, 9, 0,
		},
		{
			"current TPS is lower than target TPS and NOT increasing and within max inflight",
			args{lastTxs: 1, currentTxs: 10, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 100, targetTPS: 100, inflight: 10, maxInflight: 100},
			false, 9, 100 * multiplicativeDecrease,
		},
		{
			"current TPS is lower than target TPS but increasing and NOT within max inflight",
			args{lastTxs: 1, currentTxs: 10, lastTs: now, nowTs: now.Add(time.Second), lastTPS: 1, targetTPS: 100, inflight: 1000, maxInflight: 100},
			false, 9, 100 * multiplicativeDecrease,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotSkip, gotCurrentTPS, gotUnboundedTPS := computeTPS(tt.args.lastTxs, tt.args.currentTxs, tt.args.lastTs, tt.args.nowTs, tt.args.lastTPS, tt.args.targetTPS, tt.args.inflight, tt.args.maxInflight, tt.args.timedout)
			assert.Equal(t, tt.wantSkip, gotSkip)
			assert.Equal(t, tt.wantCurrentTPS, gotCurrentTPS)
			assert.Equal(t, tt.wantUnboundedTPS, gotUnboundedTPS)
		})
	}
}
