package metrics

import (
	// _ "github.com/dgraph-io/badger/v2" // TODO this might be needed for servers just testing metrics
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterBadgerMetrics() {
	expvarCol := prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
		"badger_disk_reads_total":     prometheus.NewDesc("badger_disk_reads_total", "cumulative number of reads", nil, nil),
		"badger_disk_writes_total":    prometheus.NewDesc("badger_disk_writes_total", "cumulative number of writes", nil, nil),
		"badger_read_bytes":           prometheus.NewDesc("badger_read_bytes", "cumulative number of bytes read", nil, nil),
		"badger_written_bytes":        prometheus.NewDesc("badger_written_bytes", "cumulative number of bytes written", nil, nil),
		"badger_gets_total":           prometheus.NewDesc("badger_gets_total", "number of gets", nil, nil),
		"badger_memtable_gets_total":  prometheus.NewDesc("badger_memtable_gets_total", "number of memtable gets", nil, nil),
		"badger_puts_total":           prometheus.NewDesc("badger_puts_total", "number of puts", nil, nil),
		"badger_blocked_puts_total":   prometheus.NewDesc("badger_blocked_puts_total", "number of blocked puts", nil, nil),
		"badger_pending_writes_total": prometheus.NewDesc("badger_pending_writes_total", "tracks the number of pending writes", []string{"path"}, nil),
		"badger_lsm_bloom_hits_total": prometheus.NewDesc("badger_lsm_bloom_hits_total", "number of LSM bloom hits", []string{"path"}, nil),
		"badger_lsm_level_gets_total": prometheus.NewDesc("badger_lsm_level_gets_total", "number of LSM gets", []string{"path"}, nil),
		"badger_lsm_size_bytes":       prometheus.NewDesc("badger_lsm_size_bytes", "size of the LSM in bytes", []string{"path"}, nil),
		"badger_vlog_size_bytes":      prometheus.NewDesc("badger_vlog_size_bytes", "size of the value log in bytes", []string{"path"}, nil),
	})

	err := prometheus.Register(expvarCol)
	if err != nil {
		panic(err)
	}
}
