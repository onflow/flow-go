package profiler

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
	"go.uber.org/atomic"
	"go.uber.org/multierr"
	pb "google.golang.org/genproto/googleapis/devtools/cloudprofiler/v2"

	"github.com/onflow/flow-go/engine"
)

var profilerEnabled atomic.Bool

// SetProfilerEnabled enable or disable generating profiler data
func SetProfilerEnabled(newState bool) {
	oldState := profilerEnabled.Swap(newState)
	if oldState != newState {
		log.Info().Bool("newState", newState).Bool("oldState", oldState).Msg("profilerEnabled changed")
	} else {
		log.Info().Bool("currentState", oldState).Msg("profilerEnabled unchanged")
	}
}

type profile struct {
	profileName string
	profileType pb.ProfileType
	profileFunc profileFunc
}

type AutoProfiler struct {
	unit     *engine.Unit
	dir      string // where we store profiles
	log      zerolog.Logger
	interval time.Duration
	duration time.Duration

	uploader Uploader
}

// New creates a new AutoProfiler instance performing profiling every interval for duration.
func New(log zerolog.Logger, uploader Uploader, dir string, interval time.Duration, duration time.Duration, enabled bool) (*AutoProfiler, error) {
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
		uploader: uploader,
	}
	return p, nil
}

func (p *AutoProfiler) Ready() <-chan struct{} {
	delay := time.Duration(float64(p.interval) * rand.Float64())
	p.unit.LaunchPeriodically(p.start, p.interval, delay)

	if profilerEnabled.Load() {
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
	if !profilerEnabled.Load() {
		return
	}

	startTime := time.Now()
	p.log.Info().Msg("starting profile trace")

	for _, prof := range [...]profile{
		{profileName: "goroutine", profileType: pb.ProfileType_THREADS, profileFunc: newProfileFunc("goroutine")},
		{profileName: "allocs", profileType: pb.ProfileType_HEAP_ALLOC, profileFunc: p.pprofAllocs},
		{profileName: "block", profileType: pb.ProfileType_CONTENTION, profileFunc: p.pprofBlock},
		{profileName: "cpu", profileType: pb.ProfileType_WALL, profileFunc: p.pprofCpu},
	} {
		path := filepath.Join(p.dir, fmt.Sprintf("%s-%s", prof.profileName, time.Now().Format(time.RFC3339)))

		logger := p.log.With().Str("profileName", prof.profileName).Str("profilePath", path).Logger()
		logger.Info().Str("file", path).Str("profile", prof.profileName).Msg("capturing")

		err := p.pprof(path, prof.profileFunc)
		if err != nil {
			logger.Warn().Err(err).Str("profile", prof.profileName).Msg("failed to generate profile")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		err = p.uploader.Upload(ctx, path, prof.profileType)
		if err != nil {
			logger.Warn().Err(err).Str("profile", prof.profileName).Msg("failed to upload profile")
			continue
		}
	}
	p.log.Info().Dur("duration", time.Since(startTime)).Msg("finished profile trace")
}

func (p *AutoProfiler) pprof(path string, profileFunc profileFunc) (err error) {
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

func (p *AutoProfiler) pprofAllocs(w io.Writer) error {
	// Forces the GC before taking each of the heap profiles and improves the profile accuracy.
	// Autoprofiler runs very infrequently so performance impact is minimal.
	runtime.GC()
	return newProfileFunc("allocs")(w)
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
