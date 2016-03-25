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

package reports

import (
	"fmt"
	"strings"
	"time"

	"github.com/sschepens/gohistogram"
	"github.com/sschepens/pla/boomer"
)

const (
	barChar = "∎"
)

type StaticReport struct {
	avgTotal float64
	fastest  float64
	slowest  float64
	average  float64
	rps      float64

	start time.Time
	total time.Duration

	errorDist      map[string]int
	statusCodeDist map[int]int
	sizeTotal      int64

	histo *gohistogram.NumericHistogram
}

func NewStaticReport() *StaticReport {
	return &StaticReport{
		start:          time.Now(),
		statusCodeDist: make(map[int]int),
		errorDist:      make(map[string]int),
		histo:          gohistogram.NewHistogram(10),
	}
}

func (r *StaticReport) ProcessResult(res boomer.Result) {
	if res.Err != nil {
		r.errorDist[res.Err.Error()]++
	} else {
		sec := res.Duration.Seconds()
		if r.slowest == 0 || sec > r.slowest {
			r.slowest = sec
		}
		if r.fastest == 0 || r.fastest > sec {
			r.fastest = sec
		}
		r.histo.Add(res.Duration.Seconds())
		r.avgTotal += res.Duration.Seconds()
		r.statusCodeDist[res.StatusCode]++
		if res.ContentLength > 0 {
			r.sizeTotal += int64(res.ContentLength)
		}
	}
}

func (r *StaticReport) Finalize() {
	r.total = time.Now().Sub(r.start)
	count := float64(r.histo.Count())
	r.rps = count / r.total.Seconds()
	r.average = r.avgTotal / count
	r.print()
}

func (r *StaticReport) print() {
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

// Prints percentile latencies.
func (r *StaticReport) printLatencies() {
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

func (r *StaticReport) printHistogram() {
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
func (r *StaticReport) printStatusCodes() {
	fmt.Printf("\nStatus code distribution:\n")
	for code, num := range r.statusCodeDist {
		fmt.Printf("  [%d]\t%d responses\n", code, num)
	}
}

func (r *StaticReport) printErrors() {
	fmt.Printf("\nError distribution:\n")
	for err, num := range r.errorDist {
		fmt.Printf("  [%d]\t%s\n", num, err)
	}
}
