package debug

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
)

var profilerEnabled bool

// SetProfilerEnabled enable or disable generating profiler data
func SetProfilerEnabled(enabled bool) {
	profilerEnabled = enabled

	log.Info().Msgf("changed profilerEnabled to %v", enabled)
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
		p.log.Info().Msgf("AutoProfiler has started, the first profiler will be genereated at: %v", time.Now().Add(p.interval))
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

	p.log.Info().Msg("starting profile trace")
	// write pprof trace files
	p.pprof("heap")
	p.pprof("goroutine")
	p.pprof("block")
	p.pprof("mutex")
	p.cpu()
	p.log.Info().Msg("finished profile trace")
}

func (p *AutoProfiler) pprof(profile string) {

	path := filepath.Join(p.dir, fmt.Sprintf("%s-%s", profile, time.Now().Format(time.RFC3339)))
	log := p.log.With().Str("file", path).Logger()
	log.Debug().Msgf("capturing %s profile", profile)

	f, err := os.Create(path)
	if err != nil {
		p.log.Error().Err(err).Msgf("failed to open %s file", profile)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Error().Err(err).Msgf("failed to close %s file", profile)
		}
	}()

	err = pprof.Lookup(profile).WriteTo(f, 0)
	if err != nil {
		p.log.Error().Err(err).Msgf("failed to write to %s file", profile)
	}
}

func (p *AutoProfiler) cpu() {
	path := filepath.Join(p.dir, fmt.Sprintf("cpu-%s", time.Now().Format(time.RFC3339)))
	log := p.log.With().Str("file", path).Logger()
	log.Debug().Msg("capturing cpu profile")

	f, err := os.Create(path)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to open cpu file")
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			p.log.Error().Err(err).Msgf("failed to close CPU file")
		}
	}()

	err = pprof.StartCPUProfile(f)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to start CPU profile")
		return
	}

	defer pprof.StopCPUProfile()

	select {
	case <-time.After(p.duration):
	case <-p.unit.Quit():
	}
}
