package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTxStatsTracker tests the TTF, TTE, TTS stats: Mean, Median, and Max.
func TestTxStatsTracker(t *testing.T) {
	st := NewTxStatsTracker(&StatsConfig{})
	tx1 := &TxStats{
		TTF:       0 * time.Second,
		TTE:       1 * time.Second,
		TTS:       2 * time.Second,
		isExpired: false,
	}
	st.AddTxStats(tx1)
	tx2 := &TxStats{
		TTF:       1 * time.Second,
		TTE:       2 * time.Second,
		TTS:       10 * time.Second,
		isExpired: false,
	}
	st.AddTxStats(tx2)
	tx3 := &TxStats{
		TTF:       100 * time.Second,
		TTE:       100 * time.Second,
		TTS:       100 * time.Second,
		isExpired: true,
	}
	st.AddTxStats(tx3)

	assert.EqualValues(t, 3, st.TotalTxSubmited())
	assert.InDelta(t, 0.3333, st.TxFailureRate(), 0.001)

	assert.InDelta(t, 1., st.TTF.Mean(), 0.1)
	assert.InDelta(t, 1.5, st.TTE.ValueAtQuantile(0.5), 0.4)
	assert.InDelta(t, 10., st.TTS.Max(), 1.)
}

// TestTxStatsTrackerString tests the String() method.
func TestTxStatsTrackerString(t *testing.T) {
	st := NewTxStatsTracker(&StatsConfig{})
	assert.Equal(t, "[]\n[]\n[]\n", st.String())

	tx1 := &TxStats{
		TTF:       1 * time.Second,
		TTE:       2 * time.Second,
		TTS:       3 * time.Second,
		isExpired: false,
	}
	st.AddTxStats(tx1)
	assert.Equal(t, "[H[1.0e+00]=1]\n[H[2.0e+00]=1]\n[H[3.0e+00]=1]\n", st.String())
}

// TestTxStatsTrackerDigest tests the Digest() method.
func TestTxStatsTrackerDigest(t *testing.T) {
	st := NewTxStatsTracker(&StatsConfig{})
	assert.Contains(t, st.Digest(), "NaN")

	tx1 := &TxStats{
		TTF:       1 * time.Second,
		TTE:       2 * time.Second,
		TTS:       3 * time.Second,
		isExpired: false,
	}
	st.AddTxStats(tx1)
	assert.Contains(t, st.Digest(), "1.05")
}
