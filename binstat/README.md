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
$ pushd binstat ; GO111MODULE=on go test -v ./... ; popd 
...
=== RUN   TestWithPprof
...
- debug: loop=0 try=1; running 6 identical functions with gomaxprocs=1
- debug: loop=0 try=2; running 6 identical functions with gomaxprocs=1
- debug: loop=0 try=3; running 6 identical functions with gomaxprocs=1
- debug: loop=1 try=1; running 6 identical functions with gomaxprocs=8
- debug: loop=1 try=2; running 6 identical functions with gomaxprocs=8
- debug: loop=1 try=3; running 6 identical functions with gomaxprocs=8
```

* The test collects running times from `binstat` & `pprof` for a side by side comparison.
  * Note: With `gomaxprocs=1` there is a large delta between CPU & wall-clock, e.g. 0.13 vs 0.84 seconds.
  * Note: With `gomaxprocs=8` `pprof` CPU time varies from 0.02 to 0.11, but  seconds.

```
- binstat------- pprof---------
- try1 try2 try3 try1 try2 try3
- 0.83 0.82 0.82 0.14 0.13 0.13 // f1() seconds; loop=0 gomaxprocs=1
- 0.81 0.80 0.80 0.15 0.14 0.15 // f2() seconds; loop=0 gomaxprocs=1
- 0.75 0.75 0.75 0.14 0.15 0.14 // f3() seconds; loop=0 gomaxprocs=1
- 0.78 0.77 0.77 0.14 0.14 0.14 // f4() seconds; loop=0 gomaxprocs=1
- 0.74 0.74 0.74 0.14 0.14 0.14 // f5() seconds; loop=0 gomaxprocs=1
- 0.84 0.84 0.77 0.13 0.13 0.13 // f6() seconds; loop=0 gomaxprocs=1
- binstat------- pprof---------
- try1 try2 try3 try1 try2 try3
- 0.15 0.15 0.15 0.05 0.05 0.02 // f1() seconds; loop=1 gomaxprocs=8
- 0.14 0.14 0.14 0.11 0.02 0.11 // f2() seconds; loop=1 gomaxprocs=8
- 0.14 0.15 0.14 0.03 0.08 0.06 // f3() seconds; loop=1 gomaxprocs=8
- 0.14 0.14 0.14 0.06 0.11 0.09 // f4() seconds; loop=1 gomaxprocs=8
- 0.15 0.15 0.14 0.06 0.08 0.04 // f5() seconds; loop=1 gomaxprocs=8
- 0.14 0.15 0.15 0.07 0.02 0.04 // f6() seconds; loop=1 gomaxprocs=8
```

* Finally, the test shows the `binstat` sorted file containing the stats.
* This part is when `GOMAXPROCS=1`.
  * Note: The `binstat.tck` bins show how many 1/10ths of a second had how many go-routines running.
  * Note: The `binstat.dmp` bins show how long opportunistically saving the `binstat` file took.

```
- debug: output of command: ls -al ./binstat.test.pid-*.txt ; cat ./binstat.test.pid-*.txt | sort --version-sort
-rw-r--r--  1 simonhardy-francis  staff  3779 18 Jun 15:31 ./binstat.test.pid-023913.binstat.txt
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f1]/file[binstat_test.run.func1:74]/time[0.200000-0.299999]=2 0.577273
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f1]/file[binstat_test.run.func1:74]/time[0.300000-0.399999]=1 0.375265
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:74]/time[0.200000-0.299999]=1 0.284086
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:74]/time[0.300000-0.399999]=2 0.701519
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:74]/time[0.200000-0.299999]=2 0.556883
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:74]/time[0.300000-0.399999]=1 0.340072
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f4]/file[binstat_test.run.func1:74]/time[0.300000-0.399999]=3 0.941300
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:74]/time[0.200000-0.299999]=3 0.829652
/GOMAXPROCS=1,CPUS=8/what[~1f-via-f6]/file[binstat_test.run.func1:74]/time[0.300000-0.399999]=3 1.163841
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:4]=5 0.000056
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:5]=1 0.000017
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:6]=1 0.000006
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:9]=1 0.000008
/GOMAXPROCS=1,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:10-19]=10 0.000138
/GOMAXPROCS=1,CPUS=8/what[internal-dump]/file[binstat.dmp:411]/time[0.080000-0.089999]=1 0.081061
/GOMAXPROCS=1,CPUS=8/what[loop-0]/file[binstat_test.TestWithPprof:169]/time[1.000000-1.999999]=1 1.882274
```

* This part is when `GOMAXPROCS=8`.
  * Note: The `f1` - `f6` functions executed in parallel without pre-emption & therefore quicker.

```
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f1]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=3 0.208766
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=2 0.138321
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f2]/file[binstat_test.run.func1:74]/time[0.070000-0.079999]=1 0.070255
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=2 0.139178
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f3]/file[binstat_test.run.func1:74]/time[0.070000-0.079999]=1 0.070239
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f4]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=3 0.208381
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=2 0.137084
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f5]/file[binstat_test.run.func1:74]/time[0.070000-0.079999]=1 0.070042
/GOMAXPROCS=8,CPUS=8/what[~1f-via-f6]/file[binstat_test.run.func1:74]/time[0.060000-0.069999]=3 0.208238
/GOMAXPROCS=8,CPUS=8/what[~2egNewTimeVal]/file[binstat.TestRng:18]/size[end:100-199]/time[0.000020-0.000029]=1 0.000023
/GOMAXPROCS=8,CPUS=8/what[~2egNewTimeVal]/file[binstat.TestRng:18]/size[pnt:myPnt]/time[0.000010-0.000019]=1 0.000010
/GOMAXPROCS=8,CPUS=8/what[~2egNew]/file[binstat.TestRng:15]/size[end:100-199]=1 0.000028
/GOMAXPROCS=8,CPUS=8/what[~2egnewTimeValInternal]/file[binstat.TestRng:22]/size[new:100-199]/time[0.000009-0.000009]=1 0.000010
/GOMAXPROCS=8,CPUS=8/what[~2egnewTimeValInternal]/file[binstat.TestRng:22]/size[pnt:myPnt]/time[0.000004-0.000004]=1 0.000005
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:4]=4 0.000120
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:5]=3 0.000085
/GOMAXPROCS=8,CPUS=8/what[internal-NumG]/file[binstat.tck:399]/size[end:10-19]=2 0.000030
/GOMAXPROCS=8,CPUS=8/what[internal-dump]/file[binstat.dmp:411]/time[0.001000-0.001999]=1 0.001428
/GOMAXPROCS=8,CPUS=8/what[loop-1]/file[binstat_test.TestWithPprof:169]/time[0.700000-0.799999]=1 0.799061
```

* This part shows internal timings, as well as the Unix time, & elapsed seconds since process start.

```
/internal/GCStats=5 0.001110
/internal/binstat.Dbg=36 0.000004
/internal/binstat.New=71 0.006490
/internal/binstat.Pnt=72 0.151686
/internal/binstat.rng=112 0.000480
/internal/second=1624055476 2.787199
```

## Environment variables

* If `BINSTAT_ENABLE` exists, `binstat` is enabled. Default: Disabled.
* If `BINSTAT_VERBOSE` exists, `binstat` outputs debug info. Default: Disabled.
* If `BINSTAT_CUT_PATH` exists, cut function name path with this. Default: `=github.com/onflow/flow-go/`.
* If `BINSTAT_LEN_WHAT` exists, truncate `<what>` to `<len>` from `~<what>=<len>`, e.g. `~f=99;~2eg=99`.

## Todo

* Monitor this [tsc github issue](https://github.com/templexxx/tsc/issues/8) in case we can accelerate timing further.
* How to best compress and access the per second `binstat` file from outside the container?
