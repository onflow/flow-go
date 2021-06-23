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

type globalStruct struct {
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
}

const (
	sizeAtEnter = iota
	sizeNotUsed
	sizeAtLeave
)

const (
	internalSec = iota
	internalNew
	internalPnt
	internalRng
	internalDbg
	internalGC
)

const maxDigitsUint64 = 20

var globalRWMutex = sync.RWMutex{}
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
	globalRWMutex.Lock() // lock for single writer <-- harmless but probably unnecessary since init() gets executed before other package functions
	defer globalRWMutex.Unlock()

	global.dumps = 0
	global.startTime = time.Now()
	global.startTimeMono = runtimeNanoAsTimeDuration()
	_, global.verbose = os.LookupEnv("BINSTAT_VERBOSE")
	_, global.enable = os.LookupEnv("BINSTAT_ENABLE")
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
			if (len(subParts) != 2) || (subParts[0][0:1] != "~") || (0 == len(subParts[0][1:])) || (0 == atoi(subParts[1])) {
				panic(fmt.Sprintf("ERROR: BINSTAT_LEN_WHAT=%s <-- cannot parse <-- format should be ~<what prefix>=<max len>[;...], e.g. ~Code=99;~X=99\n", global.lenWhat))
			}
			k := subParts[0][1:]
			v := atoi(subParts[1])
			global.what2len[k] = v
			if global.verbose {
				elapsedThisProc := time.Duration(runtimeNanoAsTimeDuration() - global.startTimeMono).Seconds()
				fmt.Printf("%f %d=pid %d=tid init() // parsing .lenWhat=%s; extracted #%d k=%s v=%d\n", elapsedThisProc, os.Getpid(), int64(C.gettid()), global.lenWhat, n, k, v)
			}
		}
	}
	global.processBaseName = filepath.Base(os.Args[0])
	global.processPid = os.Getpid()
	global.key2index = make(map[string]int)

	appendInternalKey := func(keyIotaIndex int, name string) {
		if keyIotaIndex != len(global.keysArray) {
			panic(fmt.Sprintf("ERROR: INTERNAL: %s", name))
		}
		global.keysArray = append(global.keysArray, name)
		global.frequency = append(global.frequency, 0)
		global.accumMono = append(global.accumMono, 0)
		global.frequencyShadow = append(global.frequencyShadow, 0)
		global.accumMonoShadow = append(global.accumMonoShadow, 0)
	}
	appendInternalKey(internalSec, "/internal/second")
	appendInternalKey(internalNew, "/internal/binstat.New")
	appendInternalKey(internalPnt, "/internal/binstat.Pnt")
	appendInternalKey(internalRng, "/internal/binstat.rng")
	appendInternalKey(internalDbg, "/internal/binstat.Dbg")
	appendInternalKey(internalGC, "/internal/GCStats")

	global.index = len(global.keysArray)
	global.indexInternalMax = len(global.keysArray)
	go tck(100 * time.Millisecond) // every 0.1 seconds

	if global.verbose {
		elapsedThisProc := time.Duration(runtimeNanoAsTimeDuration() - global.startTimeMono).Seconds()
		fmt.Printf("%f %d=pid %d=tid init() // .enable=%t .verbose=%t .cutPath=%s .lenWhat=%s\n", elapsedThisProc, os.Getpid(), int64(C.gettid()), global.enable, global.verbose, global.cutPath, global.lenWhat)
	}
}

func Fin() {
	dmp()
	// todo: consider closing down more somehow?
}

func newGeneric(what string, callerParams string, callerTime bool, callerSize int64, callerSizeWhen int, verbose bool) *BinStat {
	if !global.enable {
		return nil
	}

	t := runtimeNanoAsTimeDuration()

	// todo: is there a way to speed up runtime.Caller() and/or runtime.FuncForPC() somehow? cache pc value or something like that?
	pc, _, fileLine, _ := runtime.Caller(2) // 2 assumes private binStat.newGeneric() called indirectly via public stub function; please see eof
	fn := runtime.FuncForPC(pc)
	funcName := fn.Name()
	funcName = strings.ReplaceAll(funcName, global.cutPath, "")

	whatLen := len(what)
	if (what[0:1] == "~") && (len(what) >= 3) {
		// come here if what is "~<default len><what>", meaning that BINSTAT_LEN_WHAT may override <default len>
		whatLenDefault := atoi(what[1:2])
		whatLen = whatLenDefault + 2
		if whatLenOverride, keyExists := global.what2len[what[2:2+whatLenDefault]]; keyExists {
			whatLen = whatLenOverride + 2
		}
		if whatLen > len(what) {
			whatLen = len(what)
		}
		//debug fmt.Printf("debug: detected ~%d%s in %s; using ~%d..\n", whatLenDefault, what[2:2 + whatLenDefault], what, whatLen)
	}

	p := BinStat{what[0:whatLen], t, funcName, fileLine, callerParams, callerTime, callerSize, callerSizeWhen}

	t2 := runtimeNanoAsTimeDuration()
	if t2 <= t {
		panic(fmt.Sprintf("ERROR: INTERNAL: t=%d but t3=%d\n", t, t2))
	}

	if verbose && global.verbose {
		elapsedThisProc := time.Duration(t2 - global.startTimeMono).Seconds()
		elapsedThisFunc := time.Duration(t2 - t).Seconds()
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // new in %f // what[%s] .NumCPU()=%d .GOMAXPROCS(0)=%d .NumGoroutine()=%d\n",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, elapsedThisFunc, what, runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalNew], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalNew], uint64(t2-t))

	return &p
}

func pntGeneric(p *BinStat, pointUnique string, callerSize int64, callerSizeWhen int, verbose bool) time.Duration {
	if !global.enable {
		return time.Duration(0)
	}

	t := runtimeNanoAsTimeDuration()

	elapsedNanoAsTimeDuration := t - p.enterTime
	if sizeAtLeave == callerSizeWhen {
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
		case sizeAtEnter:
			keySizeRange = fmt.Sprintf("/size[new:%s]", rng(float64(p.callerSize), true))
		case sizeNotUsed:
			keySizeRange = ""
		case sizeAtLeave:
			keySizeRange = fmt.Sprintf("/size[end:%s]", rng(float64(p.callerSize), true))
		}
	case "":
	default:
		pointType = "pnt"
		keySizeRange = fmt.Sprintf("/size[pnt:%s]", pointUnique)
	}
	if p.callerTime {
		elapsedSeconds := elapsedNanoAsTimeDuration.Seconds()
		keyTimeRange = fmt.Sprintf("/time[%s]", rng(elapsedSeconds, false))
	}
	key := fmt.Sprintf("/GOMAXPROCS=%d,CPUS=%d/what[%s]/file[%s:%d]%s%s", runtime.GOMAXPROCS(0), runtime.NumCPU(), p.what, p.callerFunc, p.callerLine, keySizeRange, keyTimeRange)

tryAgainRaceCondition:
	var frequency uint64
	var accumMono uint64
	globalRWMutex.RLock() // lock for many readers v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
	index, keyExists := global.key2index[key]
	if keyExists {
		frequency = atomic.AddUint64(&global.frequency[index], 1)
		accumMono = atomic.AddUint64(&global.accumMono[index], uint64(elapsedNanoAsTimeDuration))
	}
	globalRWMutex.RUnlock() // unlock for many readers ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^

	if keyExists {
		// full thru
	} else {
		// come here to create new hash table bucket
		globalRWMutex.Lock() // lock for single writer v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v
		_, keyExists = global.key2index[key]
		if keyExists { // come here if another func beat us to key creation
			globalRWMutex.Unlock() // unlock for single writer ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~
			goto tryAgainRaceCondition
		}
		// come here to create key and associated counter array element
		index = global.index
		global.key2index[key] = index // https://stackoverflow.com/questions/36167200/how-safe-are-golang-maps-for-concurrent-read-write-operations
		global.keysArray = append(global.keysArray, key)
		global.frequency = append(global.frequency, 1)
		global.accumMono = append(global.accumMono, uint64(elapsedNanoAsTimeDuration))
		global.index++
		globalRWMutex.Unlock() // unlock for single writer ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
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
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // %s in %f // %s=[%d]=%d %f%s\n",
			elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, pointType, elapsedSinceNew, key, index, frequency, time.Duration(accumMono).Seconds(), hint)
	}

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalPnt], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalPnt], uint64(t2-t))

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

	vInt64 := int64(v * 1000000)         // e.g.  1234567
	vString := fmt.Sprintf("%d", vInt64) // e.g. "1234567"
	var vaBytes [maxDigitsUint64]byte
	var vbBytes [maxDigitsUint64]byte
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
	atomic.AddUint64(&global.frequencyShadow[internalRng], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalRng], uint64(t2-t))

	return returnString
}

func dbgGeneric(p *BinStat, debugText string, verbose bool) {
	if !global.enable {
		return
	}

	t := runtimeNanoAsTimeDuration()

	if verbose && global.verbose {
		elapsedThisProc := time.Duration(t - global.startTimeMono).Seconds()
		fmt.Printf("%f %d=pid %d=tid %s:%d(%s) // dbg %s\n", elapsedThisProc, os.Getpid(), int64(C.gettid()), p.callerFunc, p.callerLine, p.callerParams, debugText)
	}

	t2 := runtimeNanoAsTimeDuration()

	// for internal accounting, atomically increment counters in (never appended to) shadow array; saving additional lock
	atomic.AddUint64(&global.frequencyShadow[internalDbg], 1)
	atomic.AddUint64(&global.accumMonoShadow[internalDbg], uint64(t2-t))
}

func tck(t time.Duration) {
	if !global.enable {
		return
	}

	ticker := time.NewTicker(t)
	for range ticker.C {
		p := newInternal("internal-NumG")
		endValInternal(p, int64(runtime.NumGoroutine()))
	}
}

// todo: would a different format, like CSV or JSON, be useful and how would it be used?
// todo: consider env var for path to bin dump file (instead of just current folder)?
// todo: if bin dump file path is /dev/shm, add paranoid checking around filling up /dev/shm?
// todo: consider reporting on num active go-routines [1] and/or SchedStats API [2] if/when available.
// [1] https://github.com/golang/go/issues/17089 "runtime: expose number of running/runnable goroutines #17089"
// [2] https://github.com/golang/go/issues/15490 "proposal: runtime: add SchedStats API #15490"
func dmp() {
	p := newTimeInternal("internal-dump")
	defer endInternal(p)

	var gcStats debug.GCStats
	debug.ReadGCStats(&gcStats)
	global.frequencyShadow[internalGC] = uint64(gcStats.NumGC)
	global.accumMonoShadow[internalGC] = uint64(gcStats.PauseTotal)

	globalRWMutex.RLock() // lock for many readers until function return
	defer globalRWMutex.RUnlock()

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

	fileTmp := fmt.Sprintf("./%s.pid-%06d.binstat.txt.tmp", global.processBaseName, global.processPid)
	fileNew := fmt.Sprintf("./%s.pid-%06d.binstat.txt", global.processBaseName, global.processPid)
	f, err := os.Create(fileTmp)
	if err != nil {
		panic(err)
	}
	for i := range global.keysArray {
		_, err := f.WriteString(fmt.Sprintf("%s=%d %f\n", global.keysArray[i], global.frequency[i], time.Duration(global.accumMono[i]).Seconds()))
		if err != nil {
			panic(err)
		}
	}
	err = f.Close()
	if err != nil {
		fmt.Printf("ERROR: .Close()=%s\n", err)
		panic(err)
	}
	err = os.Rename(fileTmp, fileNew) // atomically rename / move on Linux :-)
	if err != nil {
		fmt.Printf("ERROR: .Rename()=%s\n", err)
		panic(err)
	}
}

// functions BEFORE go fmt v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v v

/*
func New               (what string, callerParams string                  ) *BinStat      { return newGeneric(what, callerParams, false, 0         , sizeNotUsed, true ) }
func newInternal       (what string                                       ) *BinStat      { return newGeneric(what, ""          , false, 0         , sizeNotUsed, false) }
func NewTime           (what string, callerParams string                  ) *BinStat      { return newGeneric(what, callerParams, true , 0         , sizeNotUsed, true ) }
func newTimeInternal   (what string                                       ) *BinStat      { return newGeneric(what, ""          , true , 0         , sizeNotUsed, false) }
func NewTimeVal        (what string, callerParams string, callerSize int64) *BinStat      { return newGeneric(what, callerParams, true , callerSize, sizeAtEnter, true ) }
func newTimeValInternal(what string,                      callerSize int64) *BinStat      { return newGeneric(what, ""          , true , callerSize, sizeAtEnter, false) }

func Pnt               (p  *BinStat, pointUnique  string                  ) time.Duration { return pntGeneric(p   , pointUnique        , 0         , sizeNotUsed, true ) }
func pntInternal       (p  *BinStat, pointUnique  string                  ) time.Duration { return pntGeneric(p   , pointUnique        , 0         , sizeNotUsed, false) }
func End               (p  *BinStat                                       ) time.Duration { return pntGeneric(p   , "Leave"            , 0         , sizeNotUsed, true ) }
func endInternal       (p  *BinStat                                       ) time.Duration { return pntGeneric(p   , "Leave"            , 0         , sizeNotUsed, false) }
func EndVal            (p  *BinStat,                      callerSize int64) time.Duration { return pntGeneric(p   , "Leave"            , callerSize, sizeAtLeave, true ) }
func endValInternal    (p  *BinStat,                      callerSize int64) time.Duration { return pntGeneric(p   , "Leave"            , callerSize, sizeAtLeave, false) }

func Dbg               (p  *BinStat, debugText    string                  )               {        dbgGeneric(p   , debugText                                   , true ) }
*/

// functions W/&W/O go fmt ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~ ~

func New(what string, callerParams string) *BinStat {
	return newGeneric(what, callerParams, false, 0, sizeNotUsed, true)
}
func newInternal(what string) *BinStat {
	return newGeneric(what, "", false, 0, sizeNotUsed, false)
}
func NewTime(what string, callerParams string) *BinStat {
	return newGeneric(what, callerParams, true, 0, sizeNotUsed, true)
}
func newTimeInternal(what string) *BinStat {
	return newGeneric(what, "", true, 0, sizeNotUsed, false)
}
func NewTimeVal(what string, callerParams string, callerSize int64) *BinStat {
	return newGeneric(what, callerParams, true, callerSize, sizeAtEnter, true)
}
func newTimeValInternal(what string, callerSize int64) *BinStat {
	return newGeneric(what, "", true, callerSize, sizeAtEnter, false)
}

func Pnt(p *BinStat, pointUnique string) time.Duration {
	return pntGeneric(p, pointUnique, 0, sizeNotUsed, true)
}
func pntInternal(p *BinStat, pointUnique string) time.Duration {
	return pntGeneric(p, pointUnique, 0, sizeNotUsed, false)
}
func End(p *BinStat) time.Duration { return pntGeneric(p, "Leave", 0, sizeNotUsed, true) }
func endInternal(p *BinStat) time.Duration {
	return pntGeneric(p, "Leave", 0, sizeNotUsed, false)
}
func EndVal(p *BinStat, callerSize int64) time.Duration {
	return pntGeneric(p, "Leave", callerSize, sizeAtLeave, true)
}
func endValInternal(p *BinStat, callerSize int64) time.Duration {
	return pntGeneric(p, "Leave", callerSize, sizeAtLeave, false)
}

func Dbg(p *BinStat, debugText string) { dbgGeneric(p, debugText, true) }

// functions AFTER  go fmt ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^ ^
