package metrics

import (
	// _ "github.com/dgraph-io/badger/v2" // TODO this might be needed for servers just testing metrics
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func RegisterBadgerMetrics() error {
	expvarCol := prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
		"badger_disk_reads_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_disk_reads_total", namespaceStorage, subsystemBadger), "cumulative number of reads", nil, nil),
		"badger_disk_writes_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_disk_writes_total", namespaceStorage, subsystemBadger), "cumulative number of writes", nil, nil),
		"badger_read_bytes": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_read_bytes", namespaceStorage, subsystemBadger), "cumulative number of bytes read", nil, nil),
		"badger_written_bytes": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_written_bytes", namespaceStorage, subsystemBadger), "cumulative number of bytes written", nil, nil),
		"badger_gets_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_gets_total", namespaceStorage, subsystemBadger), "number of gets", nil, nil),
		"badger_memtable_gets_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_memtable_gets_total", namespaceStorage, subsystemBadger), "number of memtable gets", nil, nil),
		"badger_puts_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_puts_total", namespaceStorage, subsystemBadger), "number of puts", nil, nil),
		// NOTE: variable exists, but not used in badger yet
		//"badger_blocked_puts_total": prometheus.NewDesc(
		//	fmt.Sprintf("%s_%s_blocked_puts_total", namespaceStorage, subsystemBadger), "number of blocked puts", nil, nil),
		"badger_pending_writes_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_badger_pending_writes_total", namespaceStorage, subsystemBadger), "tracks the number of pending writes", []string{"path"}, nil),
		"badger_lsm_bloom_hits_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_lsm_bloom_hits_total", namespaceStorage, subsystemBadger), "number of LSM bloom hits", []string{"level"}, nil),
		"badger_lsm_level_gets_total": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_lsm_level_gets_total", namespaceStorage, subsystemBadger), "number of LSM gets", []string{"level"}, nil),
		"badger_lsm_size_bytes": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_lsm_size_bytes", namespaceStorage, subsystemBadger), "size of the LSM in bytes", []string{"path"}, nil),
		"badger_vlog_size_bytes": prometheus.NewDesc(
			fmt.Sprintf("%s_%s_vlog_size_bytes", namespaceStorage, subsystemBadger), "size of the value log in bytes", []string{"path"}, nil),
	})

	err := prometheus.Register(expvarCol)
	if err != nil {
		return fmt.Errorf("failed to register badger metrics: %w", err)
	}
	return nil
}
