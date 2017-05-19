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

package interfaces

import (
	"fmt"
	"strings"
	"time"

	"github.com/sschepens/gohistogram"
	"github.com/sschepens/pb"
	"github.com/mercadolibre/pla/boomer"
)

const (
	barChar = "∎"
)

// BasicInterface is Pla's default text-based terminal interface.
type BasicInterface struct {
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

	boom  *boomer.Boomer
	histo *gohistogram.NumericHistogram
	bar   *pb.ProgressBar
	pct   int
}

// NewBasicInterface instantiates a new BasicInterface.
func NewBasicInterface() *BasicInterface {
	return &BasicInterface{
		start:          time.Now(),
		statusCodeDist: make(map[int]int),
		errorDist:      make(map[string]int),
		histo:          gohistogram.NewHistogram(10),
	}
}

// Start initializes interface
func (b *BasicInterface) Start(boom *boomer.Boomer) {
	b.boom = boom
	b.initProgressBar()
}

// ProcessResult increments ProgressBar and keeps track of statistics.
func (b *BasicInterface) ProcessResult(res boomer.Result) {
	if res.Err != nil {
		b.errorDist[res.Err.Error()]++
	} else {
		sec := res.Duration.Seconds()
		if b.slowest == 0 || sec > b.slowest {
			b.slowest = sec
		}
		if b.fastest == 0 || b.fastest > sec {
			b.fastest = sec
		}
		b.histo.Add(res.Duration.Seconds())
		b.avgTotal += res.Duration.Seconds()
		b.statusCodeDist[res.StatusCode]++
		if res.ContentLength > 0 {
			b.sizeTotal += int64(res.ContentLength)
		}
	}
	if b.boom.Duration == 0 {
		b.bar.Increment()
	}
}

// End finishes interface.
func (b *BasicInterface) End() {
	b.bar.Finish()
	b.total = time.Now().Sub(b.start)
	count := float64(b.histo.Count())
	b.rps = count / b.total.Seconds()
	b.average = b.avgTotal / count
	b.print()
}

func (b *BasicInterface) initProgressBar() {
	if b.boom.Duration > 0 {
		b.bar = pb.New(100)
		b.pct = 0
		ticker := time.NewTicker(b.boom.Duration / 100)
		go func() {
			for range ticker.C {
				if b.pct < 100 {
					b.pct += 1
					b.bar.Increment()
				}
			}
		}()
	} else {
		b.bar = pb.New(int(b.boom.N))
	}
	b.bar.BarStart = "Pl"
	b.bar.BarEnd = "!"
	b.bar.Empty = " "
	b.bar.Current = "a"
	b.bar.CurrentN = "a"
	b.bar.Start()
}

func (b *BasicInterface) print() {
	if b.histo.Count() > 0 {
		fmt.Printf("\nSummary:\n")
		fmt.Printf("  Total:\t%4.4f secs.\n", b.total.Seconds())
		fmt.Printf("  Slowest:\t%4.4f secs.\n", b.slowest)
		fmt.Printf("  Fastest:\t%4.4f secs.\n", b.fastest)
		fmt.Printf("  Average:\t%4.4f secs.\n", b.average)
		fmt.Printf("  Requests/sec:\t%4.4f\n", b.rps)
		if b.sizeTotal > 0 {
			fmt.Printf("  Total Data Received:\t%d bytes.\n", b.sizeTotal)
			fmt.Printf("  Response Size per Request:\t%d bytes.\n", b.sizeTotal/int64(b.histo.Count()))
		}
		b.printStatusCodes()
		b.printHistogram()
		b.printLatencies()
	}

	if len(b.errorDist) > 0 {
		b.printErrors()
	}
}

// Prints percentile latencies.
func (b *BasicInterface) printLatencies() {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	fmt.Printf("\nLatency distribution:\n")
	cent := float64(100)
	for _, p := range pctls {
		q := b.histo.Quantile(float64(p) / cent)
		if q > 0 {
			fmt.Printf("  %v%% in %4.4f secs.\n", p, q)
		}
	}
}

func (b *BasicInterface) printHistogram() {
	fmt.Printf("\nResponse time histogram:\n")
	bins := b.histo.Bins()
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
func (b *BasicInterface) printStatusCodes() {
	fmt.Printf("\nStatus code distribution:\n")
	for code, num := range b.statusCodeDist {
		fmt.Printf("  [%d]\t%d responses\n", code, num)
	}
}

func (b *BasicInterface) printErrors() {
	fmt.Printf("\nError distribution:\n")
	for err, num := range b.errorDist {
		fmt.Printf("  [%d]\t%s\n", num, err)
	}
}
