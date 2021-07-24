# `binstat` - run-time & space efficient stats in bins

## Why use `binstat`?

* `pprof` reports the seconds of CPU used and not the seconds of wall-clock used.
* `trace` can be too heavyweight for larger Golang programs.
* prometheus can be heavyweight, and it's difficult to programmatically manage stats once exported.
* logging can be heavyweight; 1M function calls == 1M log lines.

## `binstat` goals

* `binstat` is disabled at compile time by default resulting in zero production code overhead.
* `binstat` may be enabled at compile time on a stat by stat basis without enabling all stats.
* `binstat` aims to be efficient at collecting small stats very fast, e.g. measuring lock time.
* `binstat` aims to allow stats to be compared programmatically; is this worse/better than last run?

## How `binstat` works?

* Sprinkle `binstat.Enter*(<what>)` and `binstat.Leave*()` into Golang source code, e.g.:

```
import (
...
	"github.com/onflow/flow-go/utils/binstat"
...
)
...
	for loop := 0; loop < 2; loop++ {
		bs := binstat.EnterTime(fmt.Sprintf("loop-%d", loop))
		myFunc() // code to generate stats for
		binstat.Leave(bs)
	}
```

* Run the program, and max. once per second a stats file will be written opportunistically, e.g.:

```
$ cat myproc.pid-085882.txt| egrep loop
/GOMAXPROCS=1,GC=100/what[loop-0]/time[1.000000-1.999999]=1 1.912292 // e.g. mypkg.myFunc:161
/GOMAXPROCS=8,GC=100/what[loop-1]/time[0.600000-0.699999]=1 0.696432 // e.g. mypkg.myFunc:161
$ #   user defined string ^^^^^^
$ #          bin time range in seconds ^^^^^^^^^^^^^^^^^
$ #                                 times bin incremented ^
$ #                        total seconds for all increments ^^^^^^^^
$ #                                         name & line of instrumented file ^^^^^^^^^^^^^^^^
```

* If `myFunc()` is identical, why different results? E.g. launches 3 go-routines executing slower if `GOMAXPROCS=1`.

### How 'collapsable' `<what>` works?

* If `<what>` has the format `~<default len><what>`, then `~4RootL1L2` only uses `~4Root` in the bin name, unless env var `BINSTAT_LEN_WHAT="~Root=6"` in which case the bin name uses `~4RootL1`.
* In this way, the bin quantity for a particular `<what>` defaults to fewer bins, but may be enlarged using the env var.

## Disable AKA 'hide' `binstat` instrumentation before pushing code

```
$ utils/binstat/binstat                     
utils/binstat/binstat: Please specifiy a command line option: (list|hide|unhide)-instrumentation [regex='binstat.Bin(Net|MakeID)']
$ utils/binstat/binstat hide-instrumentation
- hide instrumentation found 17 files to examine <-- source files containing binstat instrumentation
- hide instrumentation wrote 0 files             <-- source files rewritten with commented binstat
```

* Auto commented `binstat` code looks like this:

```
import (
...
	_ "github.com/onflow/flow-go/utils/binstat"
...
)
...
	for loop := 0; loop < 2; loop++ {
		//bs := binstat.EnterTime(fmt.Sprintf("loop-%d", loop))
		myFunc() // code to generate stats for
		//binstat.Leave(bs)
	}
```

## What makes `binstat` efficient?

* The number of bins is relatively small, regardless of the number of function calls.
* For timing, `binstat` uses `runtime.nano()` which is used by & about twice as fast as `time.Now()`.
* A lock per stat collection is eliminated using a `sync.RWMutex{}` reader/writer mutual exclusion lock:
  * Usual case: If the bin already exists, any 'reader' can *concurrently* update it via atomic operations.
  * Rare event: Else a single 'writer' blocks all 'readers' to add the new bin.
* `binstat` instrumentation can be disabled in two different ways:
  * At compile time via source code commenting of instrumentation.
  * At run-time if `BINSTAT_ENABLE` does not exist.

## Example comparison with `pprof`

* This example -- on a GCP Linux box -- launches 6 identical go-routines, 3 times, with `gomaxprocs=1` & then `=8`.

```
$ pushd binstat ; GO111MODULE=on go test -v -vv ./... 2>&1 | perl -lane 's~\\n~\n~g; s~"time".*?,~~g; print;' ; popd
...
=== RUN   TestWithPprof
...
{"level":"debug","message":"test: loop=0 try=1; running 6 identical functions with gomaxprocs=1"}
...
{"level":"debug","message":"test: loop=0 try=2; running 6 identical functions with gomaxprocs=1"}
...
{"level":"debug","message":"test: loop=0 try=3; running 6 identical functions with gomaxprocs=1"}
...
{"level":"debug","message":"test: loop=1 try=1; running 6 identical functions with gomaxprocs=8"}
...
{"level":"debug","message":"test: loop=1 try=2; running 6 identical functions with gomaxprocs=8"}
...
{"level":"debug","message":"test: loop=1 try=3; running 6 identical functions with gomaxprocs=8"}
...
```

* The test collects running times from `binstat` & `pprof` for a side by side comparison.
  * Note: With `gomaxprocs=1` there is a large delta between CPU & wall-clock, e.g. 0.07 vs 0.40 seconds.
  * Note: With `gomaxprocs=8` `pprof` CPU time incorrectly varies between 0.02 to 0.12 seconds.

```
{"level":"debug","message":"test: binstat------- pprof---------"}
{"level":"debug","message":"test: try1 try2 try3 try1 try2 try3"}
{"level":"debug","message":"test: 0.38 0.40 0.38 0.06 0.06 0.05 // f1() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: 0.37 0.38 0.36 0.06 0.07 0.06 // f2() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: 0.29 0.37 0.34 0.06 0.06 0.06 // f3() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: 0.33 0.35 0.32 0.06 0.07 0.06 // f4() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: 0.28 0.34 0.29 0.05 0.06 0.06 // f5() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: 0.40 0.40 0.36 0.06 0.05 0.06 // f6() seconds; loop=0 gomaxprocs=1"}
{"level":"debug","message":"test: binstat------- pprof---------"}
{"level":"debug","message":"test: try1 try2 try3 try1 try2 try3"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.04 0.05 0.07 // f1() seconds; loop=1 gomaxprocs=8"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.05 0.07 0.05 // f2() seconds; loop=1 gomaxprocs=8"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.06 0.11 0.02 // f3() seconds; loop=1 gomaxprocs=8"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.06 0.06 0.06 // f4() seconds; loop=1 gomaxprocs=8"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.09 0.05 0.04 // f5() seconds; loop=1 gomaxprocs=8"}
{"level":"debug","message":"test: 0.07 0.08 0.07 0.03 0.04 0.12 // f6() seconds; loop=1 gomaxprocs=8"}
```

* Finally, the test shows the `binstat` sorted file containing the stats.
* This part is when `GOMAXPROCS=1`.
  * Note: The `binstat.tck` bins show how many 1/10ths of a second had how many go-routines running.
  * Note: The `binstat.dmp` bins show how long opportunistically saving the `binstat` file took.

```
{"level":"debug","message":"test: output of command: ls -al ./binstat.test.pid-*.binstat.txt ; cat ./binstat.test.pid-*.binstat.txt | sort --version-sort
-rw-r--r--  1 simonhardy-francis  staff  3610 23 Jun 17:10 ./binstat.test.pid-047616.binstat.txt
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f1]/time[0.200000-0.299999]=1 0.292582 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f1]/time[0.300000-0.399999]=2 0.758238 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f2]/time[0.200000-0.299999]=2 0.587235 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f2]/time[0.300000-0.399999]=1 0.356942 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f3]/time[0.300000-0.399999]=3 1.017089 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f4]/time[0.200000-0.299999]=2 0.562171 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f4]/time[0.300000-0.399999]=1 0.315660 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f5]/time[0.200000-0.299999]=2 0.567996 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f5]/time[0.300000-0.399999]=1 0.301534 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f6]/time[0.300000-0.399999]=3 1.157075 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/size[4]=6 0.000185 // e.g. utils/binstat.tick:424
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/size[5]=2 0.000042 // e.g. utils/binstat.tick:424
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/size[9]=2 0.000012 // e.g. utils/binstat.tick:424
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/size[10-19]=9 0.000137 // e.g. utils/binstat.tick:424
/GOMAXPROCS=1,CPUS=8/what[internal-dump]/time[0.000600-0.000699]=1 0.000650 // e.g. utils/binstat.dump:436
/GOMAXPROCS=1,CPUS=8/what[loop-0]/time[1.000000-1.999999]=1 1.812913 // e.g. utils/binstat_test.TestWithPprof:176
```

* This part is when `GOMAXPROCS=8`.
  * Note: The `f1` - `f6` functions executed in parallel without pre-emption & therefore quicker.

```
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f1]/time[0.070000-0.079999]=2 0.145029 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f1]/time[0.080000-0.089999]=1 0.082286 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f2]/time[0.070000-0.079999]=2 0.144383 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f2]/time[0.100000-0.199999]=1 0.100980 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f3]/time[0.070000-0.079999]=2 0.146867 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f3]/time[0.100000-0.199999]=1 0.106264 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f4]/time[0.070000-0.079999]=2 0.144903 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f4]/time[0.090000-0.099999]=1 0.098164 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f5]/time[0.070000-0.079999]=2 0.144949 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f5]/time[0.090000-0.099999]=1 0.098013 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f6]/time[0.070000-0.079999]=2 0.141520 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f6]/time[0.100000-0.199999]=1 0.103813 // e.g. utils/binstat_test.run.func1:83
/GOMAXPROCS=8,CPUS=8/what[~2egEnterTimeVal]/size[100-199]/time[0.000010-0.000019]=1 0.000017 // e.g. utils/binstat.TestBinstatInternal:20
/GOMAXPROCS=8,CPUS=8/what[~2egEnterTimeVal]/size[myPoint]/time[0.000005-0.000005]=1 0.000006 // e.g. utils/binstat.TestBinstatInternal:20
/GOMAXPROCS=8,CPUS=8/what[~2egEnter]/size[100-199]=1 0.000070 // e.g. utils/binstat.TestBinstatInternal:17
/GOMAXPROCS=8,CPUS=8/what[~2egenterTimeValInternal]/size[100-199]/time[0.000004-0.000004]=1 0.000005 // e.g. utils/binstat.TestBinstatInternal:24
/GOMAXPROCS=8,CPUS=8/what[~2egenterTimeValInternal]/size[myPoint]/time[0.000002-0.000002]=1 0.000002 // e.g. utils/binstat.TestBinstatInternal:24
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/size[4]=5 0.000160 // e.g. utils/binstat.tick:424
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/size[5]=1 0.000029 // e.g. utils/binstat.tick:424
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/size[10-19]=2 0.000046 // e.g. utils/binstat.tick:424
/GOMAXPROCS=8,CPUS=8/what[internal-dump]/time[0.000900-0.000999]=1 0.000943 // e.g. utils/binstat.dump:436
/GOMAXPROCS=8,CPUS=8/what[loop-1]/time[0.800000-0.899999]=1 0.819436 // e.g. utils/binstat_test.TestWithPprof:176
```

* This part shows internal timings, as well as the Unix time, elapsed seconds since process start, & the number of GCs and the total seconds they took..

```
/internal/GCStats=9 0.001566
/internal/binstat.debug=36 0.001022
/internal/binstat.enter=70 0.001932
/internal/binstat.point=71 0.000882
/internal/binstat.x_2_y=111 0.000383
/internal/second=1624493407 2.665259
"}
```

## Environment variables

* If `BINSTAT_ENABLE` exists, `binstat` is enabled. Default: Disabled.
* If `BINSTAT_VERBOSE` exists, `binstat` outputs debug info & e.g. function location. Default: Disabled.
* If `BINSTAT_DMP_NAME` exists, use it. Default: `<process name>.pid-<process pid>.binstat.txt`.
* If `BINSTAT_DMP_PATH` exists, output dump to `<BINSTAT_DMP_PATH>/<BINSTAT_DMP_NAME>`. Default: `.`.
* If `BINSTAT_CUT_PATH` exists, cut function name path with this. Default: `=github.com/onflow/flow-go/`.
* If `BINSTAT_LEN_WHAT` exists, truncate `<what>` to `<len>` from `~<what>=<len>`, e.g. `~f=99;~2eg=0`.

## Todo

* Monitor this [tsc github issue](https://github.com/templexxx/tsc/issues/8) in case we can accelerate timing further.
* How to best compress & access the per second `binstat` file from outside the container?
