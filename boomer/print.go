// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package boomer

import (
	"fmt"
	"github.com/sschepens/gohistogram"
	"strings"
	"sync"
	"time"
)

const (
	barChar = "∎"
)

type report struct {
	avgTotal float64
	fastest  float64
	slowest  float64
	average  float64
	rps      float64

	results chan *result
	start   time.Time
	total   time.Duration

	errorDist      map[string]int
	statusCodeDist map[int]int
	sizeTotal      int64

	output string

	wg    *sync.WaitGroup
	histo *gohistogram.NumericHistogram
}

func newReport(size int, results chan *result, output string) *report {
	wg := &sync.WaitGroup{}
	r := &report{
		output:         output,
		results:        results,
		start:          time.Now(),
		statusCodeDist: make(map[int]int),
		errorDist:      make(map[string]int),
		wg:             wg,
		histo:          gohistogram.NewHistogram(10),
	}
	wg.Add(1)
	go r.process()
	return r
}

func (r *report) process() {
	for res := range r.results {
		if res.err != nil {
			r.errorDist[res.err.Error()]++
		} else {
			sec := res.duration.Seconds()
			if r.slowest == 0 || sec > r.slowest {
				r.slowest = sec
			}
			if r.fastest == 0 || r.fastest > sec {
				r.fastest = sec
			}
			r.histo.Add(res.duration.Seconds())
			r.avgTotal += res.duration.Seconds()
			r.statusCodeDist[res.statusCode]++
			if res.contentLength > 0 {
				r.sizeTotal += int64(res.contentLength)
			}
		}
	}
	r.wg.Done()
}

func (r *report) finalize() {
	r.wg.Wait()
	r.total = time.Now().Sub(r.start)
	count := float64(r.histo.Count())
	r.rps = count / r.total.Seconds()
	r.average = r.avgTotal / count
	r.print()
}

func (r *report) print() {
	if r.output == "csv" {
		r.printCSV()
		return
	}

	if r.histo.Count() > 0 {
		fmt.Printf("\nSummary:\n")
		fmt.Printf("  Total:\t%4.4f secs.\n", r.total.Seconds())
		fmt.Printf("  Slowest:\t%4.4f secs.\n", r.slowest)
		fmt.Printf("  Fastest:\t%4.4f secs.\n", r.fastest)
		fmt.Printf("  Average:\t%4.4f secs.\n", r.average)
		fmt.Printf("  Requests/sec:\t%4.4f\n", r.rps)
		if r.sizeTotal > 0 {
			fmt.Printf("  Total Data Received:\t%d bytes.\n", r.sizeTotal)
			fmt.Printf("  Response Size per Request:\t%d bytes.\n", r.sizeTotal/int64(r.histo.Count()))
		}
		r.printStatusCodes()
		r.printHistogram()
		r.printLatencies()
	}

	if len(r.errorDist) > 0 {
		r.printErrors()
	}
}

func (r *report) printCSV() {
	//for i, val := range r.lats {
	//	fmt.Printf("%v,%4.4f\n", i+1, val)
	//}
}

// Prints percentile latencies.
func (r *report) printLatencies() {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	fmt.Printf("\nLatency distribution:\n")
	cent := float64(100)
	for _, p := range pctls {
		q := r.histo.Quantile(float64(p) / cent)
		if q > 0 {
			fmt.Printf("  %v%% in %4.4f secs.\n", p, q)
		}
	}
}

func (r *report) printHistogram() {
	fmt.Printf("\nResponse time histogram:\n")
	bins := r.histo.Bins()
	max := bins[0].Count
	for i := 1; i < len(bins); i++ {
		if bins[i].Count > max {
			max = bins[i].Count
		}
	}
	for i := 0; i < len(bins); i++ {
		// Normalize bar lengths.
		var barLen uint64
		if max > 0 {
			barLen = bins[i].Count * 40 / max
		}
		fmt.Printf("  %4.3f [%v]\t|%v\n", bins[i].Value, bins[i].Count, strings.Repeat(barChar, int(barLen)))
	}
}

// Prints status code distribution.
func (r *report) printStatusCodes() {
	fmt.Printf("\nStatus code distribution:\n")
	for code, num := range r.statusCodeDist {
		fmt.Printf("  [%d]\t%d responses\n", code, num)
	}
}

func (r *report) printErrors() {
	fmt.Printf("\nError distribution:\n")
	for err, num := range r.errorDist {
		fmt.Printf("  [%d]\t%s\n", num, err)
	}
}
