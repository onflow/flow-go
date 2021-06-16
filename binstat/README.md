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

* Sprinkle `binstat.New*()` and `binstat.End*()` into Golang source code, e.g.:

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
- debug: output of command: ls -al ./binstat.test.pid-*.txt ; cat ./binstat.test.pid-*.txt | sort
-rw-r--r-- 1 simon_hardy_francis_dapperlabs_c simon_hardy_francis_dapperlabs_c 2723 Jun 15 18:51 ./binstat.test.pid-023263.txt
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f1]/file[binstat_test.run.func1:72]/time[0.800000-0.899999]=3 2.467652
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f2]/file[binstat_test.run.func1:72]/time[0.800000-0.899999]=3 2.414340
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f3]/file[binstat_test.run.func1:72]/time[0.700000-0.799999]=3 2.240119
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f4]/file[binstat_test.run.func1:72]/time[0.700000-0.799999]=3 2.322448
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f5]/file[binstat_test.run.func1:72]/time[0.700000-0.799999]=3 2.209643
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f6]/file[binstat_test.run.func1:72]/time[0.700000-0.799999]=1 0.767226
/CPUS=16,GOMAXPROCS=1,GC=100/what[f-via-f6]/file[binstat_test.run.func1:72]/time[0.800000-0.899999]=2 1.674665
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:10-19]=23 0.000164
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:4]=5 0.000089
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:8]=1 0.000006
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:9]=1 0.000004
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-dump]/file[binstat.dmp:380]/time[0.000100-0.000199]=2 0.000335
/CPUS=16,GOMAXPROCS=1,GC=100/what[internal-dump]/file[binstat.dmp:380]/time[0.000200-0.000299]=1 0.000214
/CPUS=16,GOMAXPROCS=1,GC=100/what[loop-0]/file[binstat_test.TestWithPprof:161]/time[3.000000-3.999999]=1 3.044985
```

* This part is when `GOMAXPROCS=8`.
  * Note: The `f1` - `f6` functions executed in parallel without pre-emption & therefore quicker.

```
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f1]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.444851
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f2]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.428401
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f3]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.432500
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f4]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.425010
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f5]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.434849
/CPUS=16,GOMAXPROCS=8,GC=100/what[f-via-f6]/file[binstat_test.run.func1:72]/time[0.100000-0.199999]=3 0.436037
/CPUS=16,GOMAXPROCS=8,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:10-19]=4 0.000073
/CPUS=16,GOMAXPROCS=8,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:4]=4 0.000064
/CPUS=16,GOMAXPROCS=8,GC=100/what[internal-NumG]/file[binstat.tck:370]/size[end:8]=1 0.000008
/CPUS=16,GOMAXPROCS=8,GC=100/what[loop-1]/file[binstat_test.TestWithPprof:161]/time[0.900000-0.999999]=1 0.940341
```

* This part shows internal timings, as well as the Unix time, & elapsed seconds since process start.

```
/internal/binstat.Dbg=36 0.000003
/internal/binstat.New=81 0.004901
/internal/binstat.Pnt=80 0.001128
/internal/binstat.rng=80 0.000665
/internal/second=1623783112 3.985671
```

## Environment variables

* If `BINSTAT_ENABLE` exists, `binstat` is enabled. Default: disabled.
* If `BINSTAT_VERBOSE` exists, `binstat` outputs debug info. Default: disabled.
* if `BINSTAT_CUT_PATH` exists, cut the function name path with this. Default: `github.com/onflow/flow-go/`.

## Todo

* Monitor this [tsc github issue](https://github.com/templexxx/tsc/issues/8) in case we can accelerate timing further.
* How to best compress and access the per second `binstat` file from outside the container?
