package metrics

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Server is the http server that will be serving the /metrics request for prometheus
type Server struct {
	server *http.Server
	log    zerolog.Logger
}

// NewServer creates a new server that will start on the specified port,
// and responds to only the `/metrics` endpoint
func NewServer(log zerolog.Logger, port uint) *Server {
	addr := ":" + strconv.Itoa(int(port))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

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

	m := &Server{
		server: &http.Server{Addr: addr, Handler: mux},
		log:    log,
	}

	return m
}

// Ready returns a channel that will close when the network stack is ready.
func (m *Server) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			// http.ErrServerClosed is returned when Close or Shutdown is called
			// we don't consider this an error, so print this with debug level instead
			if errors.Is(err, http.ErrServerClosed) {
				m.log.Debug().Err(err).Msg("metrics server shutdown")
			} else {
				m.log.Err(err).Msg("error shutting down metrics server")
			}
		}
	}()
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (m *Server) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.server.Shutdown(ctx)
		cancel()
		close(done)
	}()
	return done
}
