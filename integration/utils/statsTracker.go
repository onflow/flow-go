package main

import (
	"sync"
	"time"
)

type StatsConfig struct {
	numCollNodes    int // number of collection nodes
	numConsNodes    int // number of consensus nodes
	numExecNodes    int // number of execution nodes
	numVerfNodes    int // number of verification nodes
	numCollClusters int // number of collection clusters
	txBatchSize     int // transaction batch size (tx sent at the same time)

}
type TxStats struct {
	TTF       time.Duration
	TTE       time.Duration
	TTS       time.Duration
	isExpired bool
}

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

// AvgTTF returns average transaction time to finality (in seconds)
func (st *StatsTracker) AvgTTF() float64 {
	st.mux.Lock()
	defer st.mux.Unlock()
	sum := float64(0)
	count := 0
	for _, t := range st.txStats {
		if !t.isExpired && t.TTF > 0 {
			sum += t.TTE.Seconds()
			count++
		}
	}
	return sum / float64(count)
}

// TODO
// MedTTF int     // Median transaction time to finality (in seconds)
// VarTTF float32 // Variance of transaction time to finality (in seconds)
// AvgTTE int     // Average transaction time to execution (in seconds)
// MedTTE int     // Median transaction time to execution (in seconds)
// VarTTE float32 // Variance of transaction time to execution (in seconds)
// AvgTTS int     // Average transaction time to seal (in seconds)
// MedTTS int     // Median transaction time to seal (in seconds)
// VarTTS float32 // Variance of transaction time to seal (in seconds)

func (st *StatsTracker) String() {

}

func (st *StatsTracker) ToFile(filePath string) {

}
