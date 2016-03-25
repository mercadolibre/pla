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

// Package boomer provides commands to run load tests and display results.
package boomer

import (
	"crypto/tls"
	"math"
	"sync"
	"time"

	"github.com/Clever/leakybucket"
	"github.com/Clever/leakybucket/memory"
	"github.com/valyala/fasthttp"
)

var client = &fasthttp.Client{
	TLSConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
	MaxConnsPerHost: math.MaxInt32,
}

type Result struct {
	Err           error
	StatusCode    int
	Duration      time.Duration
	ContentLength int
}

type Boomer struct {
	// Request is the request to be made.
	Request *fasthttp.Request

	// Timeout in seconds.
	Timeout time.Duration

	// C is the concurrency level, the number of concurrent workers to run.
	C uint

	// N is the total number of requests to make.
	N uint

	// Duration is the amount of time the test should run.
	Duration time.Duration

	bucket  leakybucket.Bucket
	results chan Result
	stop    chan struct{}
	jobs    chan *fasthttp.Request
	running bool
	wg      *sync.WaitGroup
}

func NewBoomer(req *fasthttp.Request) *Boomer {
	return &Boomer{
		Request: req,
		results: make(chan Result),
		stop:    make(chan struct{}),
		jobs:    make(chan *fasthttp.Request),
		wg:      &sync.WaitGroup{},
	}
}

func (b *Boomer) WithTimeout(t time.Duration) *Boomer {
	b.Timeout = t
	return b
}

func (b *Boomer) WithAmount(n uint) *Boomer {
	if n > 0 {
		b.Duration = 0
	}
	b.N = n
	return b
}

func (b *Boomer) WithDuration(d time.Duration) *Boomer {
	if b.running {
		panic("Cannot modify boomer while running")
	}
	if d > 0 {
		b.N = 0
	}
	b.Duration = d
	return b
}

func (b *Boomer) WithRateLimit(n uint, rate time.Duration) *Boomer {
	if n > 0 {
		b.bucket, _ = memory.New().Create("pla", n, rate)
	}
	return b
}

func (b *Boomer) WithConcurrency(c uint) *Boomer {
	if b.running {
		panic("Cannot modify boomer while running")
	}
	b.C = c
	b.results = make(chan Result, c)
	return b
}

// Results returns receive-only channel of results
func (b *Boomer) Results() <-chan Result {
	return b.results
}

// Stop indicates Boomer to stop processing new requests
func (b *Boomer) Stop() {
	if !b.running {
		return
	}
	b.running = false
	close(b.stop)
}

// Wait blocks until Boomer successfully finished or is fully stopped
func (b *Boomer) Wait() {
	b.wg.Wait()
	close(b.results)
}

// Run makes all the requests, prints the summary. It blocks until
// all work is done.
func (b *Boomer) Run() {
	if b.running {
		return
	}
	b.running = true
	if b.Duration > 0 {
		time.AfterFunc(b.Duration, func() {
			b.Stop()
		})
	}
	b.runWorkers()
}

func (b *Boomer) runWorkers() {
	b.wg.Add(int(b.C))

	var i uint
	for i = 0; i < b.C; i++ {
		go b.runWorker()
	}

	b.wg.Add(1)
	go b.triggerLoop()
}

func (b *Boomer) runWorker() {
	resp := fasthttp.AcquireResponse()
	req := fasthttp.AcquireRequest()
	for r := range b.jobs {
		req.Reset()
		resp.Reset()
		r.CopyTo(req)
		s := time.Now()

		var code int
		var size int

		var err error
		if b.Timeout > 0 {
			err = client.DoTimeout(req, resp, b.Timeout)
		} else {
			err = client.Do(req, resp)
		}
		if err == nil {
			size = resp.Header.ContentLength()
			code = resp.Header.StatusCode()
		}

		b.notifyResult(code, size, err, time.Now().Sub(s))
	}
	fasthttp.ReleaseResponse(resp)
	fasthttp.ReleaseRequest(req)
	b.wg.Done()
}

func (b *Boomer) notifyResult(code int, size int, err error, d time.Duration) {
	b.results <- Result{
		StatusCode:    code,
		Duration:      d,
		Err:           err,
		ContentLength: size,
	}
}

func (b *Boomer) checkRateLimit() error {
	if b.bucket == nil {
		return nil
	}
	_, err := b.bucket.Add(1)
	return err
}

func (b *Boomer) triggerLoop() {
	defer b.wg.Done()
	defer close(b.jobs)

	var i uint
	for {
		if b.Duration == 0 && i >= b.N {
			return
		}
		select {
		case <-b.stop:
			return
		default:
			err := b.checkRateLimit()
			if err == nil {
				b.jobs <- b.Request
				i++
			} else {
				time.Sleep(b.bucket.Reset().Sub(time.Now()))
			}
		}
	}
}
