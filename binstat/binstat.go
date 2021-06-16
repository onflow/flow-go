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
)

// todo: consider using something faster than runtime.nano(), eg. https://github.com/templexxx/tsc/issues/8 <-- but maybe not safe with multiple CPUs? https://stackoverflow.com/questions/3388134/rdtsc-accuracy-across-cpu-cores
// "Another concern is that if a thread is migrated on a different processor between two measurements, the counter might skip too much or even “go back”. " https://coherent-labs.com/posts/timestamps-for-performance-measurements/

type BinStatGlobal struct {
	initialized      bool
	dumps            uint64
	cutPath          string
	processBaseName  string
	processPid       int
	index            int
	indexInternalMax int
	key2index        map[string]int
	keysArray        []string
	frequency        []uint64
	frequencyShadow  []uint64
	accumMono        []uint64
	accumMonoShadow  []uint64
	verbose          bool
	enable           bool
	second           uint64
	startTime        time.Time
	startTimeMono    time.Duration
}

type BinStat struct {
	what string
	//	enterTime      time.Time
	enterTime      time.Duration
	callerFunc     string
	callerLine     int
	callerParams   string
	callerTime     bool
	callerSize     int64
	callerSizeWhen int
}

const (
	BinStatSizeAtEnter = iota
	BinStatSizeNotUsed
	BinStatSizeAtLeave
)

const (
	BinStatInternalSec = iota
	BinStatInternalNew
	BinStatInternalPnt
	BinStatInternalRng
	BinStatInternalDbg
)

const BinStatMaxDigitsUint64 = 20

var binStatGlobalRWMutex = sync.RWMutex{}
var binStatGlobal = BinStatGlobal{}

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

func runtimeNanoAsTimeDuration() time.Duration {
	return time.Duration(runtimeNano())
}

func ini() {
	binStatGlobalRWMutex.Lock() // lock for single writer
	if false == binStatGlobal.initialized {
		binStatGlobal.dumps = 0
		binStatGlobal.startTime = time.Now()
		binStatGlobal.startTimeMono = runtimeNanoAsTimeDuration()
		_, binStatGlobal.verbose = os.LookupEnv("BINSTAT_VERBOSE")
		_, binStatGlobal.enable = os.LookupEnv("BINSTAT_ENABLE")
		binStatGlobal.cutPath, _ = os.LookupEnv("BINSTAT_CUT_PATH")
		if binStatGlobal.cutPath == "" {
			binStatGlobal.cutPath = "github.com/onflow/flow-go/"
		}
		binStatGlobal.processBaseName = filepath.Base(os.Args[0])
		binStatGlobal.processPid = os.Getpid()
		binStatGlobal.key2index = make(map[string]int)

		appendInternalKey := func(keyIotaIndex int, name string) {
			if keyIotaIndex != len(binStatGlobal.keysArray) {
				panic(fmt.Sprintf("ERROR: INTERNAL: %s", name))
			}
			binStatGlobal.keysArray = append(binStatGlobal.keysArray, name)
			binStatGlobal.frequency = append(binStatGlobal.frequency, 0)
			binStatGlobal.accumMono = append(binStatGlobal.accumMono, 0)
			binStatGlobal.frequencyShadow = append(binStatGlobal.frequencyShadow, 0)
			binStatGlobal.accumMonoShadow = append(binStatGlobal.accumMonoShadow, 0)
		}
		appendInternalKey(BinStatInternalSec, "/internal/second")
		appendInternalKey(BinStatInternalNew, "/internal/binstat.New")
		appendInternalKey(BinStatInternalPnt, "/internal/binstat.Pnt")
		appendInternalKey(BinStatInternalRng, "/internal/binstat.rng")
		appendInternalKey(BinStatInternalDbg, "/internal/binstat.Dbg")

		binStatGlobal.index = len(binStatGlobal.keysArray)
		binStatGlobal.indexInternalMax = len(binStatGlobal.keysArray)
		binStatGlobal.initialized = true
		go tck(100 * time.Millisecond) // every 0.1 seconds

		if binStatGlobal.verbose {
			elapsedThisProc := time.Duration(runtimeNanoAsTimeDuration() - binStatGlobal.startTimeMono).Seconds()
			fmt.Printf("%f %d=pid %d=tid ini() // .enable=%t .verbose=%t .cutPath=%s\n", elapsedThisProc, os.Getpid(), int64(C.gettid()), binStatGlobal.enable, binStatGlobal.verbose, binStatGlobal.cutPath)
		}

	}
	binStatGlobalRWMutex.Unlock()
}

func Fin() {
	dmp()
	// todo: consider closing down more somehow?
}

func newGeneric(what string, callerParams string, callerTime bool, callerSize int64, callerSizeWhen int, verbose bool) *BinStat {
	t := runtimeNanoAsTimeDuration()

	if false == binStatGlobal.initialized {
		ini()
	}

	if false == binStatGlobal.enable {
		return nil
	}

	// todo: is there a way to speed up runtime.Caller() and/or runtime.FuncForPC() somehow? cache pc value or something like that?
	pc, _, fileLine, _ := runtime.Caller(2) // 2 assumes binStat.NewGeneric() called indirectly
	fn := runtime.FuncForPC(pc)
	funcName := fn.Name()
	funcName = strings.ReplaceAll(funcName, binStatGlobal.cutPath, "")

	p := BinStat{what, t, funcName, fileLine, callerParams, callerTime, callerSize, callerSizeWhen}

	t2 := runtimeNanoAsTimeDuration()
	if t2 <= t {
		panic(fmt.Sprintf("ERROR: INTERNAL: t=%d but t3=%d\n", t, t2))
	}

	if verbose && binStatGlobal.verbose {
		elapsedThisProc := time.Duration(t2 - binStatGlobal.startTimeMono).Seconds()
		elapsedThisFunc := time.Duration(t2 - t).Seconds()
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // new in %f // what[%s] .NumCPU()=%d .GOMAXPROCS(0)=%d .NumGoroutine()=%d\n",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, elapsedThisFunc, what, runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&binStatGlobal.frequencyShadow[BinStatInternalNew], 1)
	atomic.AddUint64(&binStatGlobal.accumMonoShadow[BinStatInternalNew], uint64(t2-t))

	return &p
}

func pntGeneric(p *BinStat, pointUnique string, callerSize int64, callerSizeWhen int, verbose bool) time.Duration {
	t := runtimeNanoAsTimeDuration()

	if false == binStatGlobal.enable {
		return t - t
	}

	elapsedNanoAsTimeDuration := t - p.enterTime
	if BinStatSizeAtLeave == callerSizeWhen {
		p.callerSizeWhen = callerSizeWhen
		p.callerSize = callerSize
	}
	var keySizeRange string = ""
	var keyTimeRange string = ""
	var pointType string = ""
	switch pointUnique {
	case "Leave":
		pointType = "end"
		switch p.callerSizeWhen {
		case BinStatSizeAtEnter:
			keySizeRange = fmt.Sprintf("/size[new:%s]", rng(float64(p.callerSize), true))
		case BinStatSizeNotUsed:
			keySizeRange = ""
		case BinStatSizeAtLeave:
			keySizeRange = fmt.Sprintf("/size[end:%s]", rng(float64(p.callerSize), true))
		}
	case "":
	default:
		pointType = "pnt"
		keySizeRange = fmt.Sprintf("/size[pnt:%s]", pointUnique)
	}
	goGC := debug.SetGCPercent(0)
	debug.SetGCPercent(goGC)
	if p.callerTime {
		elapsedSeconds := elapsedNanoAsTimeDuration.Seconds()
		keyTimeRange = fmt.Sprintf("/time[%s]", rng(elapsedSeconds, false))
	}
	key := fmt.Sprintf("/CPUS=%d,GOMAXPROCS=%d,GC=%d/what[%s]/file[%s:%d]%s%s", runtime.NumCPU(), runtime.GOMAXPROCS(0), goGC, p.what, p.callerFunc, p.callerLine, keyTimeRange, keySizeRange)

TRY_AGAIN_RACE_CONDITION:
	var frequency uint64
	var accumMono uint64
	binStatGlobalRWMutex.RLock() // lock for many readers v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
	index, keyExists := binStatGlobal.key2index[key]
	if keyExists {
		frequency = atomic.AddUint64(&binStatGlobal.frequency[index], 1)
		accumMono = atomic.AddUint64(&binStatGlobal.accumMono[index], uint64(elapsedNanoAsTimeDuration))
	}
	binStatGlobalRWMutex.RUnlock() // unlock for many readers ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^

	if keyExists {
		// full thru
	} else {
		// come here to create new hash table bucket
		binStatGlobalRWMutex.Lock() // lock for single writer v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
		index, keyExists = binStatGlobal.key2index[key]
		if keyExists { // come here if another func beat us to key creation
			binStatGlobalRWMutex.Unlock() // unlock for single writer ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~
			goto TRY_AGAIN_RACE_CONDITION
		}
		// come here to create key and associated counter array element
		index = binStatGlobal.index
		binStatGlobal.key2index[key] = index // https://stackoverflow.com/questions/36167200/how-safe-are-golang-maps-for-concurrent-read-write-operations
		binStatGlobal.keysArray = append(binStatGlobal.keysArray, key)
		binStatGlobal.frequency = append(binStatGlobal.frequency, 1)
		binStatGlobal.accumMono = append(binStatGlobal.accumMono, uint64(elapsedNanoAsTimeDuration))
		binStatGlobal.index++
		binStatGlobalRWMutex.Unlock() // unlock for single writer ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
		frequency = 1
		accumMono = uint64(elapsedNanoAsTimeDuration)
	}
	tOld := uint64(0)
	tNew := uint64(0)
	if verbose { // come here if NOT internal instrumentation, e.g. not binstat.dmp() etc
		tNew = uint64(time.Duration(t - binStatGlobal.startTimeMono).Seconds())
		tOld = atomic.SwapUint64(&binStatGlobal.second, tNew)
	}
	t2 := runtimeNanoAsTimeDuration()
	if verbose && binStatGlobal.verbose {
		hint := ""
		if tNew > tOld {
			binStatGlobal.dumps++
			hint = fmt.Sprintf("; dump #%d", binStatGlobal.dumps)
		}
		elapsedThisProc := time.Duration(t2 - binStatGlobal.startTimeMono).Seconds()
		elapsedSinceNew := elapsedNanoAsTimeDuration.Seconds()
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // %s in %f // %s=[%d]=%d %f%s\n",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, pointType, elapsedSinceNew, key, index, frequency, time.Duration(accumMono).Seconds(), hint)
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&binStatGlobal.frequencyShadow[BinStatInternalPnt], 1)
	atomic.AddUint64(&binStatGlobal.accumMonoShadow[BinStatInternalPnt], uint64(t2-t))

	if tNew > tOld {
		// come here if won lottery to save binStats this second
		dmp()
	}

	return elapsedNanoAsTimeDuration
}

// todo: there must be a better / faster way to do all the operations below :-)
// todo: allow configuration for more granular ranges, e.g. 1.100000-1.199999, or bigger ranges e.g. 0.200000-0.399999 ?
// todo: consider outputting int range in hex for 16 bins instead of 10 bins (at a particular magnitude)?
func rng(v float64, isInt bool) string { // e.g. 1.234567
	t := runtimeNanoAsTimeDuration()

	if false == binStatGlobal.initialized {
		panic("ERROR: INTERNAL: binstat.rng() called but false == binStatGlobal.initialized\n")
	}

	vInt64 := int64(v * 1000000)         // e.g.  1234567
	vString := fmt.Sprintf("%d", vInt64) // e.g. "1234567"
	var vaBytes [BinStatMaxDigitsUint64]byte
	var vbBytes [BinStatMaxDigitsUint64]byte
	copy(vaBytes[:], []byte(vString)) // e.g. [49 50 51 52 53 54 55]
	copy(vbBytes[:], []byte(vString)) // e.g. [49 50 51 52 53 54 55]
	for i := 1; i < len(vString); i++ {
		vaBytes[i] = 48 // AKA '0'                     // e.g. [49 48 48 48 48 48 48]
		vbBytes[i] = 57 // AKA '9'                     // e.g. [49 57 57 57 57 57 57]
	}
	vaString := string(vaBytes[0:len(vString)])      // e.g. "1000000"
	vbString := string(vbBytes[0:len(vString)])      // e.g. "1999999"
	vaInt64, _ := strconv.ParseInt(vaString, 10, 64) // e.g.  1000000
	vbInt64, _ := strconv.ParseInt(vbString, 10, 64) // e.g.  1999999
	vaFloat64 := float64(vaInt64) / 1000000          // e.g. 1.000000
	vbFloat64 := float64(vbInt64) / 1000000          // e.g. 1.999999
	//fmt.Printf("debug: v=%f -> vInt64=%d -> vString=%s -> vaBytes(%T)=%+v -> vaString=%s -> vaInt64=%d err=%+v -> vaFloat64=%f\n", v, vInt64, vString, vaBytes, vaBytes, vaString, vaInt64, err, vaFloat64)
	var returnString string
	if isInt {
		if int64(vaFloat64) == int64(vbFloat64) {
			returnString = fmt.Sprintf("%d", int64(vaFloat64))
		} else {
			returnString = fmt.Sprintf("%d-%d", int64(vaFloat64), int64(vbFloat64))
		}
	} else {
		returnString = fmt.Sprintf("%f-%f", vaFloat64, vbFloat64)
	}

	t2 := runtimeNanoAsTimeDuration()

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&binStatGlobal.frequencyShadow[BinStatInternalRng], 1)
	atomic.AddUint64(&binStatGlobal.accumMonoShadow[BinStatInternalRng], uint64(t2-t))

	return returnString
}

func dbgGeneric(p *BinStat, debugText string, verbose bool) {
	t := runtimeNanoAsTimeDuration()

	if false == binStatGlobal.enable {
		return
	}

	t2 := runtimeNanoAsTimeDuration()

	if verbose && binStatGlobal.verbose {
		elapsedThisProc := time.Duration(t2 - binStatGlobal.startTimeMono).Seconds()
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // dbg %s\n", elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, debugText)
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&binStatGlobal.frequencyShadow[BinStatInternalDbg], 1)
	atomic.AddUint64(&binStatGlobal.accumMonoShadow[BinStatInternalDbg], uint64(t2-t))
}

func tck(t time.Duration) {
	if false == binStatGlobal.enable {
		return
	}

	for range time.Tick(t) {
		p := newInternal("internal-NumG")
		endValInternal(p, int64(runtime.NumGoroutine()))
		// todo: consider reporting on debug.ReadGCStats() ?
	}
}

// todo: would a different format, like CSV or JSON, be useful and how would it be used?
// todo: consider env var for path to bin dump file (instead of just current folder)?
// todo: if bin dump file path is /dev/shm, add paranoid checking around filling up /dev/shm?
func dmp() {
	p := newTimeInternal("internal-dump")
	defer endInternal(p)

	binStatGlobalRWMutex.RLock() // lock for many readers v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v

	// todo: copy into buffer and then write to file outside of reader lock?

	t := time.Now()
	seconds := uint64(t.Unix())
	binStatGlobal.frequencyShadow[BinStatInternalSec] = seconds
	binStatGlobal.accumMonoShadow[BinStatInternalSec] = uint64(runtimeNanoAsTimeDuration() - binStatGlobal.startTimeMono)

	// copy internal accounting from never-appending shadow to appending non-shadow counters
	for i := 0; i < binStatGlobal.indexInternalMax; i++ {
		v1 := atomic.LoadUint64(&binStatGlobal.frequencyShadow[i])
		atomic.StoreUint64(&binStatGlobal.frequency[i], v1)
		v2 := atomic.LoadUint64(&binStatGlobal.accumMonoShadow[i])
		atomic.StoreUint64(&binStatGlobal.accumMono[i], v2)
	}

	fileTmp := fmt.Sprintf("./%s.pid-%06d.txt.tmp", binStatGlobal.processBaseName, binStatGlobal.processPid)
	fileNew := fmt.Sprintf("./%s.pid-%06d.txt", binStatGlobal.processBaseName, binStatGlobal.processPid)
	f, err := os.Create(fileTmp)
	if err != nil {
		panic(err)
	}
	for i, _ := range binStatGlobal.keysArray {
		_, err := f.WriteString(fmt.Sprintf("%s=%d %f\n", binStatGlobal.keysArray[i], binStatGlobal.frequency[i], time.Duration(binStatGlobal.accumMono[i]).Seconds()))
		if err != nil {
			panic(err)
		}
	}
	f.Close()
	err = os.Rename(fileTmp, fileNew) // atomically rename / move on Linux :-)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		panic(err)
	}

	binStatGlobalRWMutex.RUnlock() // unlock for many readers ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
}

// functions BEFORE go fmt v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v

/*
func New               (what string, callerParams string                  ) *BinStat      { return newGeneric(what, callerParams, false, 0         , BinStatSizeNotUsed, true ) }
func newInternal       (what string                                       ) *BinStat      { return newGeneric(what, ""          , false, 0         , BinStatSizeNotUsed, false) }
func NewTime           (what string, callerParams string                  ) *BinStat      { return newGeneric(what, callerParams, true , 0         , BinStatSizeNotUsed, true ) }
func newTimeInternal   (what string                                       ) *BinStat      { return newGeneric(what, ""          , true , 0         , BinStatSizeNotUsed, false) }
func NewTimeVal        (what string, callerParams string, callerSize int64) *BinStat      { return newGeneric(what, callerParams, true , callerSize, BinStatSizeAtEnter, true ) }
func newTimeValInternal(what string,                      callerSize int64) *BinStat      { return newGeneric(what, ""          , true , callerSize, BinStatSizeAtEnter, false) }

func Pnt               (p  *BinStat, pointUnique  string                  ) time.Duration { return pntGeneric(p   , pointUnique        , 0         , BinStatSizeNotUsed, true ) }
func pntInternal       (p  *BinStat, pointUnique  string                  ) time.Duration { return pntGeneric(p   , pointUnique        , 0         , BinStatSizeNotUsed, false) }
func End               (p  *BinStat                                       ) time.Duration { return pntGeneric(p   , "Leave"            , 0         , BinStatSizeNotUsed, true ) }
func endInternal       (p  *BinStat                                       ) time.Duration { return pntGeneric(p   , "Leave"            , 0         , BinStatSizeNotUsed, false) }
func EndVal            (p  *BinStat,                      callerSize int64) time.Duration { return pntGeneric(p   , "Leave"            , callerSize, BinStatSizeAtLeave, true ) }
func endValInternal    (p  *BinStat,                      callerSize int64) time.Duration { return pntGeneric(p   , "Leave"            , callerSize, BinStatSizeAtLeave, false) }

func Dbg               (p  *BinStat, debugText    string                  )               {        dbgGeneric(p   , debugText                                          , true ) }
*/

// functions W/&W/O go fmt ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~

func New(what string, callerParams string) *BinStat {
	return newGeneric(what, callerParams, false, 0, BinStatSizeNotUsed, true)
}
func newInternal(what string) *BinStat {
	return newGeneric(what, "", false, 0, BinStatSizeNotUsed, false)
}
func NewTime(what string, callerParams string) *BinStat {
	return newGeneric(what, callerParams, true, 0, BinStatSizeNotUsed, true)
}
func newTimeInternal(what string) *BinStat {
	return newGeneric(what, "", true, 0, BinStatSizeNotUsed, false)
}
func NewTimeVal(what string, callerParams string, callerSize int64) *BinStat {
	return newGeneric(what, callerParams, true, callerSize, BinStatSizeAtEnter, true)
}
func newTimeValInternal(what string, callerSize int64) *BinStat {
	return newGeneric(what, "", true, callerSize, BinStatSizeAtEnter, false)
}

func Pnt(p *BinStat, pointUnique string) time.Duration {
	return pntGeneric(p, pointUnique, 0, BinStatSizeNotUsed, true)
}
func pntInternal(p *BinStat, pointUnique string) time.Duration {
	return pntGeneric(p, pointUnique, 0, BinStatSizeNotUsed, false)
}
func End(p *BinStat) time.Duration { return pntGeneric(p, "Leave", 0, BinStatSizeNotUsed, true) }
func endInternal(p *BinStat) time.Duration {
	return pntGeneric(p, "Leave", 0, BinStatSizeNotUsed, false)
}
func EndVal(p *BinStat, callerSize int64) time.Duration {
	return pntGeneric(p, "Leave", callerSize, BinStatSizeAtLeave, true)
}
func endValInternal(p *BinStat, callerSize int64) time.Duration {
	return pntGeneric(p, "Leave", callerSize, BinStatSizeAtLeave, false)
}

func Dbg(p *BinStat, debugText string) { dbgGeneric(p, debugText, true) }

// functions AFTER  go fmt ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
