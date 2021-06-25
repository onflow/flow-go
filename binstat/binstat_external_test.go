package binstat_test

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/binstat"
	"github.com/onflow/flow-go/utils/unittest"
)

/*
 * NOTE: This command line can be used during binstat development to:
 * 1. Run go fmt on the binstat .go files, and
 * 2. Run the linter on the binstat .go files, and
 * 3. Run the binstat tests with the full amount of logging (-v -vv), and
 * 4. Turn JSON log line output with embedded \n into real new lines, and
 * 5. Strip "time" field from JSON log line output for shorter read, and
 * 6. Show the amount of code coverage from the tests.
 *
 * pushd binstat ; go fmt ./*.go ; golangci-lint run && GO111MODULE=on go test -v -vv -coverprofile=coverage.txt -covermode=atomic --tags relic ./... | perl -lane 's~\\n~\n~g; s~"time".*?,~~g; print;' ; go tool cover -func=coverage.txt ; popd
 */

/*
 * NOTE: The code below is inspired by the goroutine.go here [1] [2].
 * [1] https://eng.uber.com/pprof-go-profiler/
 * [2] https://github.com/chabbimilind/GoPprofDemo/blob/master/goroutine.go
 */

const loops = 2
const tries = 3
const funcs = 6
const mechs = 2

var wg sync.WaitGroup
var el [loops][tries][mechs][funcs]string
var zlog zerolog.Logger

// each function f1-f6 runs the same function f and adds its wall-clock execution time to a table of elapsed times
func f1(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}
func f2(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}
func f3(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}
func f4(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}
func f5(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}
func f6(outerFuncName string, f func(string) time.Duration, loop int, try int, i int) {
	defer wg.Done()
	el[loop][try][0][i] = fmt.Sprintf("%.02f", f(outerFuncName).Seconds())
}

func run(t *testing.T, loop int, try int, gomaxprocs int) {
	pprofFileName := fmt.Sprintf("binstat_external_test.loop-%d.try-%d.gomaxprocs-%d.pprof.txt", loop, try, gomaxprocs)
	timerFile, err := os.Create(pprofFileName)
	require.NoError(t, err)

	require.NoError(t, pprof.StartCPUProfile(timerFile))

	// this function is purely for chewing CPU
	f := func(outerFuncName string) time.Duration {
		p := binstat.EnterTime(outerFuncName, "")
		var sum int
		for i := 0; i < 10000000; i++ {
			sum -= i / 2
			sum *= i
			sum /= i/3 + 1
			sum -= i / 4
		}
		binstat.Debug(p, fmt.Sprintf("%s() = %d", outerFuncName, sum))
		return binstat.Leave(p)
	}

	runtime.GOMAXPROCS(gomaxprocs)
	wg.Add(6)
	go f1("~1f-via-f1", f, loop, try, 0)
	go f2("~1f-via-f2", f, loop, try, 1)
	go f3("~1f-via-f3", f, loop, try, 2)
	go f4("~1f-via-f4", f, loop, try, 3)
	go f5("~1f-via-f5", f, loop, try, 4)
	go f6("~1f-via-f6", f, loop, try, 5)

	wg.Wait()
	pprof.StopCPUProfile()
	require.NoError(t, timerFile.Close())

	// run pprof and capture its output
	/*
		e.g. $ go tool pprof -top -unit seconds binstat_external_test.loop-1.try-2.gomaxprocs-8.pprof.txt 2>&1 | egrep '(binstat_test.f|cum)'
		e.g.      flat  flat%   sum%        cum   cum%
		e.g.         0     0%   100%      0.07s 19.44%  github.com/onflow/flow-go/binstat_test.f1
		e.g.         0     0%   100%      0.02s  5.56%  github.com/onflow/flow-go/binstat_test.f2
		e.g.         0     0%   100%      0.06s 16.67%  github.com/onflow/flow-go/binstat_test.f3
		e.g.         0     0%   100%      0.11s 30.56%  github.com/onflow/flow-go/binstat_test.f4
		e.g.         0     0%   100%      0.06s 16.67%  github.com/onflow/flow-go/binstat_test.f5
		e.g.         0     0%   100%      0.03s  8.33%  github.com/onflow/flow-go/binstat_test.f6

		$ # todo: consider workaround: have seen pprof fail on macOS extremely infrequently, e.g. below .f5 completely missing?! how?!
		$ go tool pprof -top -unit seconds binstat_external_test.loop-1.try-2.gomaxprocs-8.pprof.txt
		Type: cpu
		Time: Jun 14, 2021 at 7:37pm (PDT)
		Duration: 200.55ms, Total samples = 0.36s (179.51%)
		Showing nodes accounting for 0.36s, 100% of 0.36s total
				flat  flat%   sum%        cum   cum%
				0.36s   100%   100%      0.36s   100%  github.com/onflow/flow-go/binstat_test.run.func1
					0     0%   100%      0.07s 19.44%  github.com/onflow/flow-go/binstat_test.f1
					0     0%   100%      0.09s 25.00%  github.com/onflow/flow-go/binstat_test.f2
					0     0%   100%      0.06s 16.67%  github.com/onflow/flow-go/binstat_test.f3
					0     0%   100%      0.08s 22.22%  github.com/onflow/flow-go/binstat_test.f4
					0     0%   100%      0.06s 16.67%  github.com/onflow/flow-go/binstat_test.f6
	*/
	command := fmt.Sprintf("go tool pprof -top -unit seconds %s 2>&1 | egrep '(binstat_test.f|cum)'", pprofFileName)
	out, err := exec.Command("bash", "-c", command).Output()
	require.NoError(t, err)
	//debug zlog.Debug().Msg(fmt.Printf("test: output of command: %s\n%s", command, out))

	// regex out the (cum)ulative column in pprof output
	r, _ := regexp.Compile(` ([0-9.]+)s`)
	matches := r.FindAllStringSubmatch(string(out), -1)
	//debug zlog.Debug().Msg(fmt.Printf("test: matches=%#v", matches)) // e.g. debug: matches=[][]string{[]string{" 0.04s", "0.04"}, []string{" 0.06s", "0.06"}, []string{" 0.08s", "0.08"}, []string{" 0.04s", "0.04"}, []string{" 0.09s", "0.09"}, []string{" 0.05s", "0.05"}}
	expected := funcs
	actual := len(matches)
	require.Equal(t, expected, actual)

	// add the regex matches to a table of elapsed times
	for i := 0; i < funcs; i++ {
		//debug zlog.Debug().Msg(fmt.Printf("test: matches[%d][1]=%s", i, matches[i][1]))
		el[loop][try][1][i] = matches[i][1]
	}
}

func init() {
	os.Setenv("BINSTAT_ENABLE", "1")
	os.Setenv("BINSTAT_VERBOSE", "1")
	os.Setenv("BINSTAT_LEN_WHAT", "~f=99;~eg=99")
}

func TestWithPprof(t *testing.T) {
	zlog = unittest.Logger()

	// delete any files hanging around from previous test run
	{
		command := "ls -al ./binstat.test.pid-*.binstat.txt ./*gomaxprocs*.pprof.txt ; rm -f ./binstat.test.pid-*.binstat.txt ./*gomaxprocs*.pprof.txt"
		out, err := exec.Command("bash", "-c", command).Output()
		require.NoError(t, err)
		zlog.Debug().Msgf("test: output of command: %s\n%s", command, out)
	}

	// run the test; loops of several tries running groups of go-routines
	for loop := 0; loop < loops; loop++ {
		gomaxprocs := 8
		if 0 == loop {
			gomaxprocs = 1
		}
		p := binstat.EnterTime(fmt.Sprintf("loop-%d", loop), "")
		for try := 0; try < tries; try++ {
			zlog.Debug().Msgf("test: loop=%d try=%d; running 6 identical functions with gomaxprocs=%d", loop, try+1, gomaxprocs)
			run(t, loop, try, gomaxprocs)
		}
		binstat.Leave(p)
	}

	// output a table of results similar to this
	/*
		- binstat------- pprof---------
		- try1 try2 try3 try1 try2 try3
		- 0.29 0.30 0.29 0.05 0.03 0.05 // f1() seconds; loop=0 gomaxprocs=1
		- 0.35 0.30 0.35 0.07 0.06 0.06 // f2() seconds; loop=0 gomaxprocs=1
		- 0.28 0.33 0.28 0.06 0.06 0.06 // f3() seconds; loop=0 gomaxprocs=1
		- 0.31 0.28 0.31 0.05 0.06 0.06 // f4() seconds; loop=0 gomaxprocs=1
		- 0.27 0.28 0.27 0.05 0.05 0.05 // f5() seconds; loop=0 gomaxprocs=1
		- 0.38 0.38 0.39 0.06 0.05 0.06 // f6() seconds; loop=0 gomaxprocs=1
		- binstat------- pprof---------
		- try1 try2 try3 try1 try2 try3
		- 0.07 0.07 0.07 0.05 0.03 0.07 // f1() seconds; loop=1 gomaxprocs=8
		- 0.07 0.07 0.07 0.05 0.04 0.03 // f2() seconds; loop=1 gomaxprocs=8
		- 0.07 0.07 0.07 0.04 0.07 0.07 // f3() seconds; loop=1 gomaxprocs=8
		- 0.07 0.07 0.07 0.05 0.02 0.08 // f4() seconds; loop=1 gomaxprocs=8
		- 0.07 0.07 0.07 0.09 0.06 0.07 // f5() seconds; loop=1 gomaxprocs=8
		- 0.07 0.07 0.07 0.04 0.10 0.03 // f6() seconds; loop=1 gomaxprocs=8
	*/
	for loop := 0; loop < loops; loop++ {
		zlog.Debug().Msg("test: binstat------- pprof---------")
		l1 := "test:"
		for r := 0; r < 2; r++ {
			for try := 0; try < tries; try++ {
				l1 = l1 + fmt.Sprintf(" try%d", try+1)
			}
		}
		zlog.Debug().Msg(l1)
		gomaxprocs := 8
		if 0 == loop {
			gomaxprocs = 1
		}
		for i := 0; i < funcs; i++ {
			l2 := "test:"
			for mech := 0; mech < mechs; mech++ {
				for try := 0; try < tries; try++ {
					l2 = l2 + fmt.Sprintf(" %s", el[loop][try][mech][i])
				}
			}
			l2 = l2 + fmt.Sprintf(" // f%d() seconds; loop=%d gomaxprocs=%d", i+1, loop, gomaxprocs)
			zlog.Debug().Msg(l2)
		}
	}

	// tell binstat to close down and write its stats file one last time
	binstat.Fin()

	// cat and sort binstat stats file
	{
		command := "ls -al ./binstat.test.pid-*.binstat.txt ; cat ./binstat.test.pid-*.binstat.txt | sort --version-sort"
		out, err := exec.Command("bash", "-c", command).Output()
		require.NoError(t, err)
		zlog.Debug().Msgf("test: output of command: %s\n%s", command, out)
	}

	// todo: add more tests? which tests?

	// if we get here then no require.NoError() calls kicked in :-)
}
