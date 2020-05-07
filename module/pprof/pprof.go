package pprof

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type Profiler struct {
	server *http.Server
	log    zerolog.Logger
}

func NewProfiler(log zerolog.Logger, port uint) *Profiler {
	addr := ":" + strconv.Itoa(int(port))

	mux := http.NewServeMux()

	mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	return &Profiler{
		server: &http.Server{Addr: addr, Handler: mux},
		log:    log,
	}
}

// Ready returns a channel that will close when the network stack is ready.
func (p *Profiler) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		if err := p.server.ListenAndServe(); err != nil {
			// http.ErrServerClosed is returned when Close or Shutdown is called
			// we don't consider this an error, so print this with debug level instead
			if errors.Is(err, http.ErrServerClosed) {
				p.log.Debug().Err(err).Msg("pprof server shutdown")
			} else {
				p.log.Err(err).Msg("error shutting down pprof server")
			}
		}
	}()
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (p *Profiler) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = p.server.Shutdown(ctx)
		cancel()
		close(done)
	}()
	return done
}
