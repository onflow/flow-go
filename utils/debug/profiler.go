package debug

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"

	"github.com/onflow/flow-go/engine"
)

var profilerEnabled bool

// SetProfilerEnabled enable or disable generating profiler data
func SetProfilerEnabled(enabled bool) {
	log.Info().Bool("newState", enabled).Bool("oldState", profilerEnabled).Msg("changed profilerEnabled")
	profilerEnabled = enabled
}

type AutoProfiler struct {
	unit     *engine.Unit
	dir      string // where we store profiles
	log      zerolog.Logger
	interval time.Duration
	duration time.Duration
}

func NewAutoProfiler(log zerolog.Logger, dir string, interval time.Duration, duration time.Duration, enabled bool) (*AutoProfiler, error) {
	SetProfilerEnabled(enabled)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("could not create profile dir %v: %w", dir, err)
	}

	p := &AutoProfiler{
		unit:     engine.NewUnit(),
		log:      log.With().Str("component", "profiler").Logger(),
		dir:      dir,
		interval: interval,
		duration: duration,
	}
	return p, nil
}

func (p *AutoProfiler) Ready() <-chan struct{} {
	delay := time.Duration(float64(p.interval) * rand.Float64())
	p.unit.LaunchPeriodically(p.start, p.interval, delay)

	if profilerEnabled {
		p.log.Info().Dur("duration", p.duration).Time("nextRunAt", time.Now().Add(p.interval)).Msg("AutoProfiler has started")
	} else {
		p.log.Info().Msg("AutoProfiler has started, profiler is disabled")
	}

	return p.unit.Ready()
}

func (p *AutoProfiler) Done() <-chan struct{} {
	return p.unit.Done()
}

func (p *AutoProfiler) start() {
	if !profilerEnabled {
		return
	}

	startTime := time.Now()
	p.log.Info().Msg("starting profile trace")

	for k, v := range map[string]profileFunc{
		"goroutine":    newProfileFunc("goroutine"),
		"heap":         newProfileFunc("heap"),
		"threadcreate": newProfileFunc("threadcreate"),
		"block":        p.pprofBlock,
		"mutex":        p.pprofMutex,
		"cpu":          p.pprofCpu,
	} {
		err := p.pprof(k, v)
		if err != nil {
			p.log.Error().Err(err).Str("profile", k).Msg("failed to generate profile")
		}
	}
	p.log.Info().Dur("duration", time.Since(startTime)).Msg("finished profile trace")
}

func (p *AutoProfiler) pprof(profile string, profileFunc profileFunc) (err error) {
	path := filepath.Join(p.dir, fmt.Sprintf("%s-%s", profile, time.Now().Format(time.RFC3339)))
	p.log.Debug().Str("file", path).Str("profile", profile).Msg("capturing")

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer func() {
		multierr.AppendInto(&err, f.Close())
	}()

	return profileFunc(f)
}

type profileFunc func(io.Writer) error

func newProfileFunc(name string) profileFunc {
	return func(w io.Writer) error {
		return pprof.Lookup(name).WriteTo(w, 0)
	}
}

func (p *AutoProfiler) pprofBlock(w io.Writer) error {
	runtime.SetBlockProfileRate(100)
	defer runtime.SetBlockProfileRate(0)

	select {
	case <-time.After(p.duration):
		return newProfileFunc("block")(w)
	case <-p.unit.Quit():
		return context.Canceled
	}
}

func (p *AutoProfiler) pprofMutex(w io.Writer) error {
	runtime.SetMutexProfileFraction(10)
	defer runtime.SetMutexProfileFraction(0)

	select {
	case <-time.After(p.duration):
		return newProfileFunc("mutex")(w)
	case <-p.unit.Quit():
		return context.Canceled
	}
}

func (p *AutoProfiler) pprofCpu(w io.Writer) error {
	err := pprof.StartCPUProfile(w)
	if err != nil {
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	defer pprof.StopCPUProfile()

	select {
	case <-time.After(p.duration):
		return nil
	case <-p.unit.Quit():
		return context.Canceled
	}
}
