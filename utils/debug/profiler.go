package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
)

type AutoProfiler struct {
	unit     *engine.Unit
	dir      string // where we store profiles
	log      zerolog.Logger
	interval time.Duration
	duration time.Duration
}

func NewAutoProfiler(log zerolog.Logger, dir string, interval time.Duration, duration time.Duration) (*AutoProfiler, error) {

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("could not create profile dir: %w", err)
	}

	p := &AutoProfiler{
		unit:     engine.NewUnit(),
		log:      log.With().Str("component", "profiler").Logger(),
		dir:      dir,
		interval: time.Minute * 3,
	}
	return p, nil
}

func (p *AutoProfiler) Ready() <-chan struct{} {
	p.unit.LaunchPeriodically(p.start, p.interval)
	return p.unit.Ready()
}

func (p *AutoProfiler) Done() <-chan struct{} {
	return p.unit.Done()
}

func (p *AutoProfiler) start() {
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
