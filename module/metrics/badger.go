package metrics

import (
	// _ "github.com/dgraph-io/badger/v2" // TODO this might be needed for servers just testing metrics
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func RegisterBadgerMetrics() {
	expvarCol := prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
		fmt.Sprintf("%s_%s_disk_reads_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_disk_reads_total", "cumulative number of reads", nil, nil),
		fmt.Sprintf("%s_%s_disk_writes_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_disk_writes_total", "cumulative number of writes", nil, nil),
		fmt.Sprintf("%s_%s_read_bytes", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_read_bytes", "cumulative number of bytes read", nil, nil),
		fmt.Sprintf("%s_%s_written_bytes", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_written_bytes", "cumulative number of bytes written", nil, nil),
		fmt.Sprintf("%s_%s_gets_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_gets_total", "number of gets", nil, nil),
		fmt.Sprintf("%s_%s_memtable_gets_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_memtable_gets_total", "number of memtable gets", nil, nil),
		fmt.Sprintf("%s_%s_puts_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_puts_total", "number of puts", nil, nil),
		fmt.Sprintf("%s_%s_blocked_puts_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_blocked_puts_total", "number of blocked puts", nil, nil),
		fmt.Sprintf("%s_%s_pending_writes_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_pending_writes_total", "tracks the number of pending writes", []string{"path"}, nil),
		fmt.Sprintf("%s_%s_lsm_bloom_hits_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_lsm_bloom_hits_total", "number of LSM bloom hits", []string{"path"}, nil),
		fmt.Sprintf("%s_%s_lsm_level_gets_total", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_lsm_level_gets_total", "number of LSM gets", []string{"path"}, nil),
		fmt.Sprintf("%s_%s_lsm_size_bytes", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_lsm_size_bytes", "size of the LSM in bytes", []string{"path"}, nil),
		fmt.Sprintf("%s_%s_vlog_size_bytes", namespaceStorage, subsystemBadger): prometheus.NewDesc(
			"badger_vlog_size_bytes", "size of the value log in bytes", []string{"path"}, nil),
	})

	err := prometheus.Register(expvarCol)
	if err != nil {
		panic(err)
	}
}
