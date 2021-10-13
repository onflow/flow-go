/*
	Package binstat implements file based code statistics in bins.

	Generate bin based statistics on code by wrapping it with binstat functions.
	Every wallclock second, binstat will output bins to a text file if changed.

	API:

	  bs := binstat.Enter[Time][Val]("<What>"[,<Val>])
	  binstat.Leave[Val](bs, [<Val>])

	Bin (single text line in output file depends on .Enter*()/.Leave*() combo) anatomy:

	  /GOMAXPROCS=8,CPUS=8/what[~4whatExampleOne]/size[<num>-<num>]/time[<num>-<num>]=<num> <num>
	                                                                                        ^^^^^ Total wallclock
	                                                                                  ^^^^^ Number of samples
	                                                               ^^^^^^^^^^^^^^^^^^ If .EnterTime*() used
	                                             ^^^^^^^^^^^^^^^^^^ If <Val> given in .Enter*Val() or .LeaveVal()
	                      ^^^^^^^^^^^^^^^^^^^^^^^ <What> string from .Enter*()
	                             ^^^^^ default <What> length to include in bin unless BINSTAT_LEN_WHAT
	  ^^^^^^^^^^^^^^^^^^^^ normally does not change

	Size and time ranges are optional and auto generated from a single value or time.
	E.g. time 0.012345 will be transformed to the range 0.010000 to 0.019999.
	E.g. value 234 will be transformed to the range 200 to 299.
	Note: Value can be optionally given with .Enter*Val() or .Leave*Val() as appropriate.

	Using different API combinations then a varity of bin formats are possible:

	  /GOMAXPROCS=8,CPUS=8/what[~7formata]=<num> <num>
	  /GOMAXPROCS=8,CPUS=8/what[~7formatb]/size[<num>-<num>]=<num> <num>
	  /GOMAXPROCS=8,CPUS=8/what[~7formatc]/time[<num>-<num>]=<num> <num>
	  /GOMAXPROCS=8,CPUS=8/what[~7formatd]/size[<num>-<num>]/time[<num>-<num>]=<num> <num>

	In this way, code can be called millions of times, but relatively few bins generated & updated.

	The number of bins used at run-time & output can be modified at process start via env vars.
	E.g. By default binstat is disabled and binstat function will just return, using little CPU.
	E.g. BINSTAT_LEN_WHAT=~what=0 means disable this particular <What> stat.
	E.g. BINSTAT_LEN_WHAT=~what=99 means create a longer <What> (therefore more bins) for this particular <What> stat.

	Note: binstat also outputs bins for its internal function statistics.

	Please see API examples below.

	Please see README for examples of commenting and uncommenting.

	Please see test for example of binstat versus pprof.
*/
package binstat

/*
#if defined(__linux__)
#include <stdint.h>
#include <unistd.h>
#include <sys/syscall.h>
#ifdef SYS_gettid
uint64_t gettid() { return syscall(SYS_gettid); }
#else
#error "SYS_gettid unavailable on this system"
#endif
#elif __APPLE__ // http://elliotth.blogspot.com/2012/04/gettid-on-mac-os.html
#include <stdint.h>
#include <pthread.h>
uint64_t gettid() { uint64_t tid; pthread_threadid_np(NULL, &tid); return tid; }
#else
#   error "Unknown platform; __linux__ or __APPLE__ supported"
#endif
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// todo: consider using something faster than runtime.nano() e.g. [1]
// But is [1] maybe not safe with multiple CPUs? [2]
// "Another concern is that if a thread is migrated on a different
//  processor between two measurements, the counter might skip too much
//  or even 'go back'." [3]
// [1] https://github.com/templexxx/tsc/issues/8
// [2] https://stackoverflow.com/questions/3388134/rdtsc-accuracy-across-cpu-cores
// [3] https://coherent-labs.com/posts/timestamps-for-performance-measurements/

type globalStruct struct {
	rwMutex          sync.RWMutex
	log              zerolog.Logger
	dumps            uint64
	dmpName          string
	dmpPath          string
	cutPath          string
	processBaseName  string
	processPid       int
	index            int
	indexInternalMax int
	key2index        map[string]int
	keysArray        []string
	keysEgLoc        []string
	frequency        []uint64
	frequencyShadow  []uint64
	accumMono        []uint64
	accumMonoShadow  []uint64
	verbose          bool
	enable           bool
	second           uint64
	startTime        time.Time
	startTimeMono    time.Duration
	lenWhat          string         // e.g. "~Code=99;~X=99"
	what2len         map[string]int // e.g. Code -> 99, X -> 99
}

type BinStat struct {
	what           string
	enterTime      time.Duration
	callerFunc     string
	callerLine     int
	callerParams   string
	callerTime     bool
	callerSize     int64
	callerSizeWhen int
	keySizeRange   string
	keyTimeRange   string
}

const (
	sizeAtEnter = iota
	sizeNotUsed
	sizeAtLeave
)

const (
	internalSec = iota
	internalEnter
	internalPoint
	internalx_2_y
	internalDebug
	internalGC
)

var global = globalStruct{}

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

func runtimeNanoAsTimeDuration() time.Duration {
	return time.Duration(runtimeNano())
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func init() {
	t1 := runtimeNanoAsTimeDuration()

	// inspired by https://github.com/onflow/flow-go/blob/master/utils/unittest/logging.go
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	global.log = zerolog.New(os.Stderr).Level(zerolog.DebugLevel).With().Timestamp().Logger()

	global.dumps = 0
	global.startTime = time.Now()
	global.startTimeMono = runtimeNanoAsTimeDuration()
	global.processBaseName = filepath.Base(os.Args[0])
	global.processPid = os.Getpid()
	_, global.verbose = os.LookupEnv("BINSTAT_VERBOSE")
	_, global.enable = os.LookupEnv("BINSTAT_ENABLE")
	global.dmpName, _ = os.LookupEnv("BINSTAT_DMP_NAME")
	if global.dmpName == "" {
		global.dmpName = fmt.Sprintf("%s.pid-%06d.binstat.txt", global.processBaseName, global.processPid)
	}
	global.dmpPath, _ = os.LookupEnv("BINSTAT_DMP_PATH")
	if global.dmpPath == "" {
		global.dmpPath = "."
	}
	global.cutPath, _ = os.LookupEnv("BINSTAT_CUT_PATH")
	if global.cutPath == "" {
		global.cutPath = "github.com/onflow/flow-go/"
	}
	global.what2len = make(map[string]int)
	global.lenWhat, _ = os.LookupEnv("BINSTAT_LEN_WHAT")
	if len(global.lenWhat) > 0 {
		parts := strings.Split(global.lenWhat, ";")
		for n, part := range parts { // e.g. "~Code=99"
			subParts := strings.Split(part, "=")
			const leftAndRightParts = 2
			if (len(subParts) != leftAndRightParts) || (subParts[0][0:1] != "~") || (0 == len(subParts[0][1:])) {
				panic(fmt.Sprintf("ERROR: BINSTAT: BINSTAT_LEN_WHAT=%s <-- cannot parse <-- format should be ~<what prefix>=<max len>[;...], e.g. ~Code=99;~X=99\n", global.lenWhat))
			}
			k := subParts[0][1:]
			v := atoi(subParts[1])
			global.what2len[k] = v
			if global.verbose {
				elapsedThisProc := time.Duration(runtimeNanoAsTimeDuration() - global.startTimeMono).Seconds()
				global.log.Debug().Msgf("%f %d=pid %d=tid init() // parsing .lenWhat=%s; extracted #%d k=%s v=%d", elapsedThisProc, os.Getpid(), int64(C.gettid()), global.lenWhat, n, k, v)
			}
		}
	}
	global.key2index = make(map[string]int)

	appendInternalKey := func(keyIotaIndex int, name string) {
		if keyIotaIndex != len(global.keysArray) {
			panic(fmt.Sprintf("ERROR: BINSTAT: INTERNAL: %s", name))
		}
		global.keysArray = append(global.keysArray, name)
		global.keysEgLoc = append(global.keysEgLoc, "")
		global.frequency = append(global.frequency, 0)
		global.accumMono = append(global.accumMono, 0)
		global.frequencyShadow = append(global.frequencyShadow, 0)
		global.accumMonoShadow = append(global.accumMonoShadow, 0)
	}
	appendInternalKey(internalSec, "/internal/second")
	appendInternalKey(internalEnter, "/internal/binstat.enter")
	appendInternalKey(internalPoint, "/internal/binstat.point")
	appendInternalKey(internalx_2_y, "/internal/binstat.x_2_y")
	appendInternalKey(internalDebug, "/internal/binstat.debug")
	appendInternalKey(internalGC, "/internal/GCStats")

	global.index = len(global.keysArray)
	global.indexInternalMax = len(global.keysArray)
	go tick(100 * time.Millisecond) // every 0.1 seconds

	if global.verbose {
		elapsedThisProc := time.Duration(runtimeNanoAsTimeDuration() - global.startTimeMono).Seconds()
		global.log.Debug().Msgf("%f %d=pid %d=tid init() // .enable=%t .verbose=%t .dmpPath=%s .dmpName=%s .cutPath=%s .lenWhat=%s",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), global.enable, global.verbose, global.dmpPath, global.dmpName, global.cutPath, global.lenWhat)
	}

	t2 := runtimeNanoAsTimeDuration()
	if t2 <= t1 {
		panic(fmt.Sprintf("ERROR: BINSTAT: INTERNAL: t1=%d but t2=%d\n", t1, t2))
	}
}

func enterGeneric(what string, callerTime bool, callerSize int64, callerSizeWhen int, verbose bool) *BinStat {
	if !global.enable {
		return &BinStat{} // so that chained functions can be called even if binstat globally disabled
	}

	t := runtimeNanoAsTimeDuration()

	funcName := ""
	fileLine := 0
	if global.verbose {
		// todo: is there a way to speed up runtime.Caller() and/or runtime.FuncForPC() somehow? cache pc value or something like that?
		pc, _, lineNum, _ := runtime.Caller(2) // 2 assumes private binStat.newGeneric() called indirectly via public stub function; please see eof
		fn := runtime.FuncForPC(pc)
		funcName = fn.Name()
		funcName = strings.ReplaceAll(funcName, global.cutPath, "")
		fileLine = lineNum
	}

	whatLen := len(what) // e.g. what = "~3net:wire"
	const tildaCharLen = 1
	const lenCharLen = 1
	const whatCharLenMin = 1
	if (what[0:1] == "~") && (len(what) >= (tildaCharLen + lenCharLen + whatCharLenMin)) {
		// come here if what is "~<default len><what>", meaning that BINSTAT_LEN_WHAT may override <default len>
		//                                     ^^^^^^ 1+ chars; alphanumeric what name which may be longer than default len
		//                        ^^^^^^^^^^^^^ 1 char; digit means default what len 1-9
		//                       ^ 1 char; tilda means use next digit as default len unless override len exists in .what2len map
		whatLenDefault := atoi(what[1:2])
		whatLen = whatLenDefault + tildaCharLen + lenCharLen
		if whatLenOverride, keyExists := global.what2len[what[tildaCharLen+lenCharLen:whatLen]]; keyExists {
			whatLen = whatLenOverride + tildaCharLen + lenCharLen
		}
		if whatLen > len(what) {
			whatLen = len(what)
		}
	}

	callerParams := ""
	bs := BinStat{what[0:whatLen], t, funcName, fileLine, callerParams, callerTime, callerSize, callerSizeWhen, "", ""}

	t2 := runtimeNanoAsTimeDuration()

	if verbose && global.verbose {
		elapsedThisProc := time.Duration(t2 - global.startTimeMono).Seconds()
		elapsedThisFunc := time.Duration(t2 - t).Seconds()
		global.log.Debug().Msgf("%f %d=pid %d=tid %s:%d(%s) // enter in %f // what[%s] .NumCPU()=%d .GOMAXPROCS(0)=%d .NumGoroutine()=%d",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), bs.callerFunc, bs.callerLine, bs.callerParams, elapsedThisFunc, what, runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalEnter], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalEnter], uint64(t2-t))

	return &bs
}

func pointGeneric(bs *BinStat, pointUnique string, callerSize int64, callerSizeWhen int, verbose bool) time.Duration {
	// if binstat disabled, or <what> disabled; "~#" == "~<default len><what>"
	if (!global.enable) || (2 == len(bs.what)) {
		return time.Duration(0)
	}

	t := runtimeNanoAsTimeDuration()

	elapsedNanoAsTimeDuration := t - bs.enterTime
	if sizeAtLeave == callerSizeWhen {
		bs.callerSizeWhen = callerSizeWhen
		bs.callerSize = callerSize
	}
	var pointType string
	switch pointUnique {
	case "Leave":
		pointType = "leave"
		switch bs.callerSizeWhen {
		case sizeAtEnter:
			bs.keySizeRange = fmt.Sprintf("/size[%s]", x_2_y(float64(bs.callerSize), true))
		case sizeNotUsed:
			bs.keySizeRange = ""
		case sizeAtLeave:
			bs.keySizeRange = fmt.Sprintf("/size[%s]", x_2_y(float64(bs.callerSize), true))
		}
	case "":
	default:
		pointType = "point"
		bs.keySizeRange = fmt.Sprintf("/size[%s]", pointUnique)
	}
	if bs.callerTime {
		elapsedSeconds := elapsedNanoAsTimeDuration.Seconds()
		bs.keyTimeRange = fmt.Sprintf("/time[%s]", x_2_y(elapsedSeconds, false))
	}
	key := fmt.Sprintf("/GOMAXPROCS=%d,CPUS=%d/what[%s]%s%s", runtime.GOMAXPROCS(0), runtime.NumCPU(), bs.what, bs.keySizeRange, bs.keyTimeRange)

tryAgainRaceCondition:
	var frequency uint64
	var accumMono uint64
	global.rwMutex.RLock() // lock for many readers v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
	index, keyExists := global.key2index[key]
	if keyExists {
		frequency = atomic.AddUint64(&global.frequency[index], 1)
		accumMono = atomic.AddUint64(&global.accumMono[index], uint64(elapsedNanoAsTimeDuration))
	}
	global.rwMutex.RUnlock() // unlock for many readers ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^

	if !keyExists {
		// come here to create new hash table bucket
		keyEgLoc := ""
		if global.verbose {
			keyEgLoc = fmt.Sprintf(" // e.g. %s:%d", bs.callerFunc, bs.callerLine)
		}
		global.rwMutex.Lock() // lock for single writer v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
		_, keyExists = global.key2index[key]
		if keyExists { // come here if another func beat us to key creation
			global.rwMutex.Unlock() // unlock for single writer ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~
			goto tryAgainRaceCondition
		}
		// come here to create key and associated counter array element
		index = global.index
		global.key2index[key] = index // https://stackoverflow.com/questions/36167200/how-safe-are-golang-maps-for-concurrent-read-write-operations
		global.keysArray = append(global.keysArray, key)
		global.keysEgLoc = append(global.keysEgLoc, keyEgLoc)
		global.frequency = append(global.frequency, 1)
		global.accumMono = append(global.accumMono, uint64(elapsedNanoAsTimeDuration))
		global.index++
		global.rwMutex.Unlock() // unlock for single writer ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
		frequency = 1
		accumMono = uint64(elapsedNanoAsTimeDuration)
	}
	tOld := uint64(0)
	tNew := uint64(0)
	if verbose { // come here if NOT internal instrumentation, e.g. not binstat.dmp() etc
		tNew = uint64(time.Duration(t - global.startTimeMono).Seconds())
		tOld = atomic.SwapUint64(&global.second, tNew)
	}
	t2 := runtimeNanoAsTimeDuration()
	if verbose && global.verbose {
		hint := ""
		if tNew > tOld {
			global.dumps++
			hint = fmt.Sprintf("; dump #%d", global.dumps)
		}
		elapsedThisProc := time.Duration(t2 - global.startTimeMono).Seconds()
		elapsedSinceNew := elapsedNanoAsTimeDuration.Seconds()
		global.log.Debug().Msgf("%f %d=pid %d=tid %s:%d(%s) // %s in %f // %s=[%d]=%d %f%s",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), bs.callerFunc, bs.callerLine, bs.callerParams, pointType, elapsedSinceNew, key, index, frequency, time.Duration(accumMono).Seconds(), hint)
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalPoint], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalPoint], uint64(t2-t))

	if tNew > tOld {
		// come here if won lottery to save binStats this second
		const UseDefaultName = ""
		Dump(UseDefaultName)
	}

	return elapsedNanoAsTimeDuration
}

// todo: there must be a better / faster way to do all the operations below :-)
// todo: allow configuration for more granular ranges, e.g. 1.100000-1.199999, or bigger ranges e.g. 0.200000-0.399999 ?
// todo: consider outputting int range in hex for 16 bins instead of 10 bins (at a particular magnitude)?
func x_2_y(v float64, isInt bool) string { // e.g. 12.345678
	t := runtimeNanoAsTimeDuration()

	var s string
	if isInt {
		s = fmt.Sprintf("%d", int64(v)) // e.g. "12"
	} else {
		s = fmt.Sprintf("%f", v) // e.g. "12.2345678"
	}
	// make lo & hi copy of strings
	lo := []byte(s)
	hi := []byte(s)
	// find first non-zero digit
	var z int
	for z = 0; z < len(lo); z++ {
		if (lo[z] != '0') && (lo[z] != '.') {
			goto foundFirstNonZeroNonDot
		}
	}
foundFirstNonZeroNonDot:
	// turn the remaining lo digits to '0' and hi digits to '1'
	z++
	for i := z; i < len(lo); i++ {
		if lo[i] != '.' {
			lo[i] = '0'
			hi[i] = '9'
		}
	}
	returnString := fmt.Sprintf("%s-%s", lo, hi)

	t2 := runtimeNanoAsTimeDuration()

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalx_2_y], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalx_2_y], uint64(t2-t))

	return returnString
}

func debugGeneric(bs *BinStat, debugText string, verbose bool) {
	if !global.enable {
		return
	}

	t := runtimeNanoAsTimeDuration()

	if verbose && global.verbose {
		elapsedThisProc := time.Duration(t - global.startTimeMono).Seconds()
		global.log.Debug().Msgf("%f %d=pid %d=tid %s:%d(%s) // debug %s", elapsedThisProc, os.Getpid(), int64(C.gettid()), bs.callerFunc, bs.callerLine, bs.callerParams, debugText)
	}

	t2 := runtimeNanoAsTimeDuration()

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalDebug], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalDebug], uint64(t2-t))
}

func tick(t time.Duration) {
	if !global.enable {
		return
	}

	ticker := time.NewTicker(t)
	for range ticker.C {
		bs := enter("internal-NumG")
		leaveVal(bs, int64(runtime.NumGoroutine()))
	}
}

func fatalPanic(err string) {
	global.log.Fatal().Msg(err)
	panic(err)
}

// todo: would a different format, like CSV or JSON, be useful and how would it be used?
// todo: consider env var for path to bin dump file (instead of just current folder)?
// todo: if bin dump file path is /dev/shm, add paranoid checking around filling up /dev/shm?
// todo: consider reporting on num active go-routines [1] and/or SchedStats API [2] if/when available.
// [1] https://github.com/golang/go/issues/17089 "runtime: expose number of running/runnable goroutines #17089"
// [2] https://github.com/golang/go/issues/15490 "proposal: runtime: add SchedStats API #15490"

// Dump binstat text file.
// Normally called automatically once per second if bins have changed.
// Consider calling explicity before exiting process.
// Called explicity in tests so as not to wait one second.
func Dump(dmpNonDefaultName string) {
	bs := enterTime("internal-dump")
	defer leave(bs)

	var gcStats debug.GCStats
	debug.ReadGCStats(&gcStats)
	global.frequencyShadow[internalGC] = uint64(gcStats.NumGC)
	global.accumMonoShadow[internalGC] = uint64(gcStats.PauseTotal)

	global.rwMutex.RLock() // lock for many readers until function return
	defer global.rwMutex.RUnlock()

	// todo: copy into buffer and then write to file outside of reader lock?

	t := time.Now()
	seconds := uint64(t.Unix())
	global.frequencyShadow[internalSec] = seconds
	global.accumMonoShadow[internalSec] = uint64(runtimeNanoAsTimeDuration() - global.startTimeMono)

	// copy internal accounting from never-appending shadow to appending non-shadow counters
	for i := 0; i < global.indexInternalMax; i++ {
		v1 := atomic.LoadUint64(&global.frequencyShadow[i])
		atomic.StoreUint64(&global.frequency[i], v1)
		v2 := atomic.LoadUint64(&global.accumMonoShadow[i])
		atomic.StoreUint64(&global.accumMono[i], v2)
	}

	fileTmp := fmt.Sprintf("%s/%s.%d.tmp%s", global.dmpPath, global.dmpName, seconds, dmpNonDefaultName)
	fileNew := fmt.Sprintf("%s/%s%s", global.dmpPath, global.dmpName, dmpNonDefaultName)
	f, err := os.Create(fileTmp)
	if err != nil {
		fatalPanic(fmt.Sprintf("ERROR: BINSTAT: .Create(%s)=%s", fileTmp, err))
	}
	for i := range global.keysArray {
		// grab these atomically (because they may be atomically updated in parallel) so as not to trigger Golang's "WARNING: DATA RACE"
		u1 := atomic.LoadUint64(&global.frequency[i])
		u2 := atomic.LoadUint64(&global.accumMono[i])
		_, err := fmt.Fprintf(f, "%s=%d %f%s\n", global.keysArray[i], u1, time.Duration(u2).Seconds(), global.keysEgLoc[i])
		if err != nil {
			fatalPanic(fmt.Sprintf("ERROR: BINSTAT: .Fprintf()=%s", err))
		}
	}
	err = f.Close()
	if err != nil {
		fatalPanic(fmt.Sprintf("ERROR: BINSTAT: .Close()=%s", err))
	}
	err = os.Rename(fileTmp, fileNew) // atomically rename / move on Linux :-)
	if err != nil {
		// sometimes -- very infrequently -- we come here with the error: "no such file or directory"
		// in theory only one go-routine should write this uniquely named per second file, so how can the file 'disappear' for renaming?
		// therefore this error results in a warning and not an error / panic, and the next second we just write the file again hopefuly :-)
		global.log.Warn().Msgf("WARN: .Rename(%s, %s)=%s\n", fileTmp, fileNew, err)
	}
}

// functions for exposing binstat internals e.g. for running non-production experiments sampling data, etc
func (bs *BinStat) GetSizeRange() string { return bs.keySizeRange }
func (bs *BinStat) GetTimeRange() string { return bs.keyTimeRange }
func (bs *BinStat) GetWhat() string      { return bs.what }

// functions BEFORE go fmt v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v

/*
func Enter       (what string,                    ) *BinStat      { return enterGeneric(what, false       , 0         , sizeNotUsed, true ) }
func enter       (what string                     ) *BinStat      { return enterGeneric(what, false       , 0         , sizeNotUsed, false) }
func EnterVal    (what string, callerSize   int64 ) *BinStat      { return enterGeneric(what, false       , callerSize, sizeAtEnter, true ) }
func enterVal    (what string, callerSize   int64 ) *BinStat      { return enterGeneric(what, false       , callerSize, sizeAtEnter, false) }
func EnterTime   (what string,                    ) *BinStat      { return enterGeneric(what, true        , 0         , sizeNotUsed, true ) }
func enterTime   (what string                     ) *BinStat      { return enterGeneric(what, true        , 0         , sizeNotUsed, false) }
func EnterTimeVal(what string, callerSize   int64 ) *BinStat      { return enterGeneric(what, true        , callerSize, sizeAtEnter, true ) }
func enterTimeVal(what string, callerSize   int64 ) *BinStat      { return enterGeneric(what, true        , callerSize, sizeAtEnter, false) }

func DebugParams (bs *BinStat, callerParams string) *BinStat      {        bs.callerParams  = callerParams; return bs                       }
func Debug       (bs *BinStat, text         string)               {        debugGeneric(bs  , text                                 , true ) }

func Point       (bs *BinStat, pointUnique  string) time.Duration { return pointGeneric(bs  , pointUnique , 0         , sizeNotUsed, true ) }
func point       (bs *BinStat, pointUnique  string) time.Duration { return pointGeneric(bs  , pointUnique , 0         , sizeNotUsed, false) }
func Leave       (bs *BinStat                     ) time.Duration { return pointGeneric(bs  , "Leave"     , 0         , sizeNotUsed, true ) }
func leave       (bs *BinStat                     ) time.Duration { return pointGeneric(bs  , "Leave"     , 0         , sizeNotUsed, false) }
func LeaveVal    (bs *BinStat, callerSize   int64 ) time.Duration { return pointGeneric(bs  , "Leave"     , callerSize, sizeAtLeave, true ) }
func leaveVal    (bs *BinStat, callerSize   int64 ) time.Duration { return pointGeneric(bs  , "Leave"     , callerSize, sizeAtLeave, false) }
*/

// functions W/&W/O go fmt ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~

func Enter(what string) *BinStat { return enterGeneric(what, false, 0, sizeNotUsed, true) }
func enter(what string) *BinStat { return enterGeneric(what, false, 0, sizeNotUsed, false) }
func EnterVal(what string, callerSize int64) *BinStat {
	return enterGeneric(what, false, callerSize, sizeAtEnter, true)
}
func enterVal(what string, callerSize int64) *BinStat {
	return enterGeneric(what, false, callerSize, sizeAtEnter, false)
}
func EnterTime(what string) *BinStat { return enterGeneric(what, true, 0, sizeNotUsed, true) }
func enterTime(what string) *BinStat { return enterGeneric(what, true, 0, sizeNotUsed, false) }
func EnterTimeVal(what string, callerSize int64) *BinStat {
	return enterGeneric(what, true, callerSize, sizeAtEnter, true)
}
func enterTimeVal(what string, callerSize int64) *BinStat {
	return enterGeneric(what, true, callerSize, sizeAtEnter, false)
}

func DebugParams(bs *BinStat, callerParams string) *BinStat {
	bs.callerParams = callerParams
	return bs
}
func Debug(bs *BinStat, text string) { debugGeneric(bs, text, true) }

func Point(bs *BinStat, pointUnique string) time.Duration {
	return pointGeneric(bs, pointUnique, 0, sizeNotUsed, true)
}
func point(bs *BinStat, pointUnique string) time.Duration {
	return pointGeneric(bs, pointUnique, 0, sizeNotUsed, false)
}
func Leave(bs *BinStat) time.Duration { return pointGeneric(bs, "Leave", 0, sizeNotUsed, true) }
func leave(bs *BinStat) time.Duration { return pointGeneric(bs, "Leave", 0, sizeNotUsed, false) }
func LeaveVal(bs *BinStat, callerSize int64) time.Duration {
	return pointGeneric(bs, "Leave", callerSize, sizeAtLeave, true)
}
func leaveVal(bs *BinStat, callerSize int64) time.Duration {
	return pointGeneric(bs, "Leave", callerSize, sizeAtLeave, false)
}

// functions AFTER  go fmt ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
