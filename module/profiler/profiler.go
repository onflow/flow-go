package profiler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/google/pprof/profile"
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

type profileDef struct {
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

	for _, prof := range [...]profileDef{
		{profileName: "goroutine", profileType: pb.ProfileType_THREADS, profileFunc: newProfileFunc("goroutine")},
		{profileName: "heap", profileType: pb.ProfileType_HEAP, profileFunc: p.pprofHeap},
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

func (p *AutoProfiler) goHeapProfile(sampleTypes ...string) (*profile.Profile, error) {
	if len(sampleTypes) == 0 {
		return nil, fmt.Errorf("no sample types specified")
	}

	// Forces the GC before taking each of the heap profiles and improves the profile accuracy.
	// Autoprofiler runs very infrequently so performance impact is minimal.
	runtime.GC()

	buf := &bytes.Buffer{}
	err := newProfileFunc("heap")(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate allocs profile: %w", err)
	}

	prof, err := profile.Parse(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse allocs profile: %w", err)
	}
	prof.TimeNanos = time.Now().UnixNano()

	selectedSampleTypes := make([]int, 0, len(sampleTypes))
	for _, name := range sampleTypes {
		for i, sampleType := range prof.SampleType {
			if sampleType.Type == name {
				selectedSampleTypes = append(selectedSampleTypes, i)
			}
		}
	}
	if len(selectedSampleTypes) != len(sampleTypes) {
		return nil, fmt.Errorf("failed to find all sample types: want: %+v, got: %+v", sampleTypes, prof.SampleType)
	}

	newSampleType := make([]*profile.ValueType, 0, len(selectedSampleTypes))
	for _, j := range selectedSampleTypes {
		newSampleType = append(newSampleType, prof.SampleType[j])
	}
	prof.SampleType = newSampleType
	prof.DefaultSampleType = prof.SampleType[0].Type

	for _, s := range prof.Sample {
		newValue := make([]int64, 0, len(selectedSampleTypes))
		for _, j := range selectedSampleTypes {
			newValue = append(newValue, s.Value[j])
		}
		s.Value = newValue
	}

	// Merge profile with itself to remove empty samples.
	prof, err = profile.Merge([]*profile.Profile{prof})
	if err != nil {
		return nil, fmt.Errorf("failed to merge profile: %w", err)
	}

	return prof, nil
}

// pprofHeap produces cumulative heap profile since the program start.
func (p *AutoProfiler) pprofHeap(w io.Writer) error {
	prof, err := p.goHeapProfile("inuse_objects", "inuse_space")
	if err != nil {
		return fmt.Errorf("failed to get heap profile: %w", err)
	}

	return prof.Write(w)
}

// pprofAllocs produces differential allocs profile for the given duration.
func (p *AutoProfiler) pprofAllocs(w io.Writer) (err error) {
	p1, err := p.goHeapProfile("alloc_objects", "alloc_space")
	if err != nil {
		return fmt.Errorf("failed to get allocs profile: %w", err)
	}

	select {
	case <-time.After(p.duration):
	case <-p.unit.Quit():
		return context.Canceled
	}

	p2, err := p.goHeapProfile("alloc_objects", "alloc_space")
	if err != nil {
		return fmt.Errorf("failed to get allocs profile: %w", err)
	}

	// multiply values by -1 and merge to get differential profile.
	p1.Scale(-1)
	diff, err := profile.Merge([]*profile.Profile{p1, p2})
	if err != nil {
		return fmt.Errorf("failed to merge allocs profiles: %w", err)
	}
	diff.TimeNanos = time.Now().UnixNano()
	diff.DurationNanos = p.duration.Nanoseconds()

	return diff.Write(w)
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
