# `binstat` - run-time & space efficient stats in bins

## Why use `binstat`?

* `pprof` reports the seconds of CPU used and not the seconds of wall-clock used.
* `trace` can be too heavyweight for larger Golang programs.
* prometheus can be heavyweight, and it's difficult to programmatically manage stats once exported.
* logging can be heavyweight; 1M function calls == 1M log lines.

## `binstat` goals

* Be able to instrument arbitrary code points without worrying about reducing run-time performance.
* Run instrumented code points arbitrary times without worrying about reducing run-time performance.
* Allow collected stats to be managed and compared programmatically; is this worse/better than last run?

## How `binstat` works?

* Sprinkle `binstat.New*(<what>)` and `binstat.End*()` into Golang source code, e.g.:

```
	for loop := 0; loop < 2; loop++ {
		p := binstat.NewTime(fmt.Sprintf("loop-%d", loop), "")
		myFunc()
		binstat.End(p)
	}
```

* Run the program, and max. once per second a stats file will be written opportunistically, e.g.:

```
$ cat myproc.pid-085882.txt| egrep loop
/GOMAXPROCS=1,GC=100/what[loop-0]/file[mypkg.myFunc:161]/time[1.000000-1.999999]=1 1.912292
/GOMAXPROCS=8,GC=100/what[loop-1]/file[mypkg.myFunc:161]/time[0.600000-0.699999]=1 0.696432
$ #   user defined string ^^^^^^
$ #   name & line of instrumented file ^^^^^^^^^^^^^^^^
$ #                                 bin time range in seconds ^^^^^^^^^^^^^^^^^
$ #                                                        times bin incremented ^
$ #                                               total seconds for all increments ^^^^^^^^
```

* If `myFunc()` is identical, why different results? E.g. launches 3 go-routines executing slower if `GOMAXPROCS=1`.

### How 'collapsable' `<what>` works?

* If `<what>` has the format `~<default len><what>`, then `~4RootL1L2` only uses `~4Root` in the bin name, unless env var `BINSTAT_LEN_WHAT="~Root=6"` in which case the bin name uses `~4RootL1`.
* In this way, the bin quantity for a particular `<what>` defaults to fewer bins, but may be enlarged using the env var.

## What makes `binstat` efficient?

* The number of bins is relatively small, regardless of the number of function calls.
* For timing, `binstat` uses `runtime.nano()` which is used by & about twice as fast as `time.Now()`.
* A lock per stat collection is eliminated using a `sync.RWMutex{}` reader/writer mutual exclusion lock:
  * Usual case: If the bin already exists, any 'reader' can *concurrently* update it via atomic operations.
  * Rare event: Else a single 'writer' blocks all 'readers' to add the new bin.

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
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f1]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=3 1.158629
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=3 1.103506
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:73]/time[0.200000-0.299999]=1 0.293651
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=2 0.703707
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f4]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=3 1.001581
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:73]/time[0.200000-0.299999]=2 0.575242
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=1 0.337427
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f6]/file[binstat_test.run.func1:73]/time[0.300000-0.399999]=1 0.357828
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f6]/file[binstat_test.run.func1:73]/time[0.400000-0.499999]=2 0.807230
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:4]=4 0.000063
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:5]=3 0.000088
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:8]=1 0.000012
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:9]=1 0.000013
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:10-19]=10 0.000146
/GOMAXPROCS=1,CPUS=8/what[internal-dump]/file[binstat.dmp:407]/time[0.000900-0.000999]=1 0.000955
/GOMAXPROCS=1,CPUS=8/what[loop-0]/file[binstat_test.TestWithPprof:166]/time[1.000000-1.999999]=1 1.842978
```

* This part is when `GOMAXPROCS=8`.
  * Note: The `f1` - `f6` functions executed in parallel without pre-emption & therefore quicker.

```
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f1]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.221967
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.218902
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.221567
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f4]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.218523
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.223819
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f6]/file[binstat_test.run.func1:73]/time[0.070000-0.079999]=3 0.223709
/GOMAXPROCS=8,CPUS=8/what[~2egNewTimeVal]/file[binstat.TestBinstatInternal:20]/size[end:100-199]/time[0.000020-0.000029]=1 0.000025
/GOMAXPROCS=8,CPUS=8/what[~2egNewTimeVal]/file[binstat.TestBinstatInternal:20]/size[pnt:myPnt]/time[0.000007-0.000007]=1 0.000007
/GOMAXPROCS=8,CPUS=8/what[~2egNew]/file[binstat.TestBinstatInternal:17]/size[end:100-199]=1 0.000061
/GOMAXPROCS=8,CPUS=8/what[~2egnewTimeValInternal]/file[binstat.TestBinstatInternal:24]/size[new:100-199]/time[0.000006-0.000006]=1 0.000006
/GOMAXPROCS=8,CPUS=8/what[~2egnewTimeValInternal]/file[binstat.TestBinstatInternal:24]/size[pnt:myPnt]/time[0.000002-0.000002]=1 0.000003
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:4]=4 0.000078
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:5]=1 0.000084
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:6]=1 0.000009
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:395]/size[end:10-19]=1 0.000273
/GOMAXPROCS=8,CPUS=8/what[internal-dump]/file[binstat.dmp:407]/time[0.000900-0.000999]=1 0.000961
/GOMAXPROCS=8,CPUS=8/what[loop-1]/file[binstat_test.TestWithPprof:166]/time[0.700000-0.799999]=1 0.752880
```

* This part shows internal timings, as well as the Unix time, elapsed seconds since process start, & the number of GCs and the total seconds they took..

```
/internal/GCStats=9 0.001566
/internal/binstat.Dbg=36 0.001022
/internal/binstat.New=70 0.001932
/internal/binstat.Pnt=71 0.000882
/internal/binstat.rng=111 0.000383
/internal/second=1624493407 2.665259
"}
```

## Environment variables

* If `BINSTAT_ENABLE` exists, `binstat` is enabled. Default: Disabled.
* If `BINSTAT_VERBOSE` exists, `binstat` outputs debug info. Default: Disabled.
* If `BINSTAT_DMP_NAME` exists, use it. Default: `<process name>.pid-<process pid>.binstat.txt`.
* If `BINSTAT_DMP_PATH` exists, output dump to `<BINSTAT_DMP_PATH>/<BINSTAT_DMP_NAME>`. Default: `.`.
* If `BINSTAT_CUT_PATH` exists, cut function name path with this. Default: `=github.com/onflow/flow-go/`.
* If `BINSTAT_LEN_WHAT` exists, truncate `<what>` to `<len>` from `~<what>=<len>`, e.g. `~f=99;~2eg=99`.

## Todo

* Monitor this [tsc github issue](https://github.com/templexxx/tsc/issues/8) in case we can accelerate timing further.
* How to best compress & access the per second `binstat` file from outside the container?
