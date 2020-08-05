/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/go-test/deep"

	"github.com/onflow/cadence-initial/runtime/parser"
	"github.com/onflow/cadence-initial/runtime/parser2"
)

var benchFlag = flag.Bool("bench", false, "benchmark the new and the old parser")
var compareFlag = flag.Bool("compare", false, "compare the results of the new and old parser")

func main() {
	flag.Parse()
	args := flag.Args()
	if *benchFlag {
		bench(args)
	}
	if *compareFlag {
		compare(args)
	}
}

func bench(args []string) {
	w := newTabWriter()

	if len(args) == 0 {
		data, err := ioutil.ReadAll(bufio.NewReader(os.Stdin))
		if err != nil {
			panic(err)
		}

		oldResult, newResult := benchOldAndNew(string(data))

		_, err = fmt.Fprintf(w, "[old]\t%s\n", oldResult)
		if err != nil {
			panic(err)
		}
		_, err = fmt.Fprintf(w, "[new]\t%s\n", newResult)
		if err != nil {
			panic(err)
		}

		return
	} else {
		for i := 0; i < len(args); i++ {
			filename := args[i]
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				panic(err)
			}

			oldResult, newResult := benchOldAndNew(string(data))

			_, err = fmt.Fprintf(w, "%s:\t[old]\t%s\n", filename, oldResult)
			if err != nil {
				panic(err)
			}
			_, err = fmt.Fprintf(w, "%s:\t[new]\t%s\n", filename, newResult)
			if err != nil {
				panic(err)
			}
		}
	}

	err := w.Flush()
	if err != nil {
		panic(err)
	}
}

func benchOldAndNew(code string) (oldResult, newResult testing.BenchmarkResult) {
	oldResult = benchParse(code, func(code string) (err error) {
		_, _, err = parser.ParseProgram(code)
		return
	})

	newResult = benchParse(code, func(code string) (err error) {
		_, err = parser2.ParseProgram(code)
		return
	})

	return
}

func benchParse(code string, parse func(string) error) testing.BenchmarkResult {
	return testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := parse(code)
			if err != nil {
				panic(err)
			}
		}
	})
}

func compare(args []string) {

	if len(args) == 0 {
		data, err := ioutil.ReadAll(bufio.NewReader(os.Stdin))
		if err != nil {
			panic(err)
		}

		diff := compareOldAndNew(string(data))

		if len(diff) == 0 {
			_, err = fmt.Printf("OK\n\n")
			if err != nil {
				panic(err)
			}
		} else {
			_, err = fmt.Printf("MISMATCH:\n%s\n", diff)
			if err != nil {
				panic(err)
			}
		}

		return
	} else {
		for i := 0; i < len(args); i++ {
			filename := args[i]
			data, err := ioutil.ReadFile(filename)
			if err != nil {
				panic(err)
			}

			diff := compareOldAndNew(string(data))

			if len(diff) == 0 {
				_, err = fmt.Printf("%s:\tOK\n\n", filename)
				if err != nil {
					panic(err)
				}
			} else {
				_, err = fmt.Printf("%s:\tMISMATCH:\n%s\n", filename, diff)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func compareOldAndNew(code string) string {
	oldResult, _, err := parser.ParseProgram(code)
	if err != nil {
		return fmt.Sprintf("old parser failed: %s", err)
	}

	newResult, err := parser2.ParseProgram(code)
	if err != nil {
		return fmt.Sprintf("new parser failed: %s", err)
	}

	// the maximum levels of a struct to recurse into
	// this prevents infinite recursion from circular references
	deep.MaxDepth = 100

	diff := deep.Equal(oldResult, newResult)

	s := strings.Builder{}

	for _, d := range diff {
		s.WriteString(d)
		s.WriteRune('\n')
	}

	return s.String()
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
}
