package utils

import (
	"sort"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/table"
)

// StatsConfig captures configuration details of an experiment
type StatsConfig struct {
	numCollNodes    int // number of collection nodes
	numConsNodes    int // number of consensus nodes
	numExecNodes    int // number of execution nodes
	numVerfNodes    int // number of verification nodes
	numCollClusters int // number of collection clusters
	txBatchSize     int // transaction batch size (tx sent at the same time)

}

// TxStats holds stats about execution of a transaction
type TxStats struct {
	TTF       time.Duration
	TTE       time.Duration
	TTS       time.Duration
	isExpired bool
}

// StatsTracker keep track of TxStats
type StatsTracker struct {
	config  *StatsConfig
	txStats []*TxStats
	mux     sync.Mutex
}

func NewStatsTracker(config *StatsConfig) *StatsTracker {
	return &StatsTracker{
		config:  config,
		txStats: make([]*TxStats, 0),
	}
}

func (st *StatsTracker) AddTxStats(tt *TxStats) {
	st.mux.Lock()
	defer st.mux.Unlock()
	st.txStats = append(st.txStats, tt)
}

// TotalTxSubmited returns the total transaction submited
func (st *StatsTracker) TotalTxSubmited() int {
	return len(st.txStats)
}

// TxFailureRate returns the number of expired transactions divided by total number of transactions
func (st *StatsTracker) TxFailureRate() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	counter := 0
	for _, t := range st.txStats {
		if t.isExpired {
			counter++
		}
	}
	return float64(counter) / float64(len(st.txStats))
}

// AvgTTF returns the average transaction time to finality (in seconds)
func (st *StatsTracker) AvgTTF() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	sum := float64(0)
	count := 0
	for _, t := range st.txStats {
		if !t.isExpired && t.TTF > 0 {
			sum += t.TTF.Seconds()
			count++
		}
	}
	return sum / float64(count)
}

// MedTTF returns the median transaction time to finality (in seconds)
func (st *StatsTracker) MedTTF() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()

	observations := make([]float64, 0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTF > 0 {
			observations = append(observations, t.TTF.Seconds())
		}
	}
	sort.Float64s(observations)

	if len(observations)%2 == 0 { // even
		return (observations[(len(observations)/2)-1] + observations[len(observations)/2]) / 2
	}
	return observations[len(observations)/2]
}

// MaxTTF returns the maximum transaction time to finality (in seconds)
func (st *StatsTracker) MaxTTF() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	max := float64(0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTF > 0 {
			if t.TTF.Seconds() > max {
				max = t.TTF.Seconds()
			}
		}
	}
	return max
}

// AvgTTE returns the average transaction time to execution (in seconds)
func (st *StatsTracker) AvgTTE() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	sum := float64(0)
	count := 0
	for _, t := range st.txStats {
		if !t.isExpired && t.TTE > 0 {
			sum += t.TTE.Seconds()
			count++
		}
	}
	return sum / float64(count)
}

// MedTTE returns the median transaction time to execution (in seconds)
func (st *StatsTracker) MedTTE() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()

	observations := make([]float64, 0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTE > 0 {
			observations = append(observations, t.TTE.Seconds())
		}
	}
	sort.Float64s(observations)

	if len(observations)%2 == 0 { // even
		return (observations[(len(observations)/2)-1] + observations[len(observations)/2]) / 2
	}
	return observations[len(observations)/2]
}

// MaxTTE returns the maximum transaction time to execution (in seconds)
func (st *StatsTracker) MaxTTE() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	max := float64(0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTE > 0 {
			if t.TTE.Seconds() > max {
				max = t.TTE.Seconds()
			}
		}
	}
	return max
}

// AvgTTS return the average transaction time to seal (in seconds)
func (st *StatsTracker) AvgTTS() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	sum := float64(0)
	count := 0
	for _, t := range st.txStats {
		if !t.isExpired && t.TTS > 0 {
			sum += t.TTS.Seconds()
			count++
		}
	}
	return sum / float64(count)
}

// MaxTTS returns the maximum transaction time to seal (in seconds)
func (st *StatsTracker) MaxTTS() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	max := float64(0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTS > 0 {
			if t.TTS.Seconds() > max {
				max = t.TTS.Seconds()
			}
		}
	}
	return max
}

// MedTTS returns the median transaction time to seal (in seconds)
func (st *StatsTracker) MedTTS() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()

	observations := make([]float64, 0)
	for _, t := range st.txStats {
		if !t.isExpired && t.TTS > 0 {
			observations = append(observations, t.TTS.Seconds())
		}
	}
	sort.Float64s(observations)

	if len(observations)%2 == 0 { // even
		return (observations[(len(observations)/2)-1] + observations[len(observations)/2]) / 2
	}
	return observations[len(observations)/2]
}

func (st *StatsTracker) String() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"total TX",
		"failure rate",
		"avg TTF",
		"median TTF",
		"max TTF",
		"avg TTE",
		"median TTE",
		"max TTE",
		"avg TTS",
		"median TTS",
		"max TTS"})
	t.AppendRow(table.Row{st.TotalTxSubmited(),
		st.TxFailureRate(),
		st.AvgTTF(),
		st.MedTTF(),
		st.MaxTTF(),
		st.AvgTTE(),
		st.MedTTE(),
		st.MaxTTE(),
		st.AvgTTS(),
		st.MedTTS(),
		st.MaxTTS()})
	return t.Render()
}
