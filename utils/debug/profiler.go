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
}

func NewAutoProfiler(dir string, log zerolog.Logger) *AutoProfiler {
	p := &AutoProfiler{
		unit:     engine.NewUnit(),
		log:      log.With().Str("debug", "auto-profiler").Logger(),
		dir:      dir,
		interval: time.Minute * 3,
	}
	return p
}

func (p *AutoProfiler) Ready() <-chan struct{} {
	p.unit.Launch(p.start)
	return p.unit.Ready()
}

func (p *AutoProfiler) Done() <-chan struct{} {
	return p.unit.Done()
}

func (p *AutoProfiler) start() {

	tick := time.NewTicker(p.interval)
	defer tick.Stop()

	for {
		select {
		case <-p.unit.Quit():
			return
		case <-tick.C:

			p.log.Debug().Msg("starting profile trace")
			// write pprof trace files
			p.pprof("heap")
			p.pprof("goroutine")
			p.pprof("block")
			p.pprof("mutex")
			p.cpu()
			p.log.Debug().Msg("finished profile trace")
		}
	}
}

func (p *AutoProfiler) pprof(profile string) {
	path := filepath.Join(p.dir, fmt.Sprintf("%s-%s", profile, time.Now().Format(time.RFC3339)))
	f, err := os.Create(path)
	if err != nil {
		p.log.Error().Err(err).Msgf("failed to open %s file", profile)
		return
	}

	err = pprof.Lookup(profile).WriteTo(f, 0)
	if err != nil {
		p.log.Error().Err(err).Msgf("failed to write to %s file", profile)
	}
}

func (p *AutoProfiler) cpu() {
	path := filepath.Join(p.dir, fmt.Sprintf("cpu-%s", time.Now().Format(time.RFC3339)))
	f, err := os.Create(path)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to open cpu file")
		return
	}

	err = pprof.StartCPUProfile(f)
	if err != nil {
		p.log.Error().Err(err).Msg("failed to start CPU profile")
		return
	}
	time.Sleep(time.Second * 10)
	pprof.StopCPUProfile()
}
