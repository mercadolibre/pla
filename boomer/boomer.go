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
	"github.com/valyala/fasthttp"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sschepens/pb"
)

type result struct {
	err           error
	statusCode    int
	duration      time.Duration
	contentLength int
}

type Boomer struct {
	// Request is the request to be made.
	Request *fasthttp.Request

	RequestBody string

	// N is the total number of requests to make.
	N int

	// C is the concurrency level, the number of concurrent workers to run.
	C int

	// Timeout in seconds.
	Timeout int

	// Qps is the rate limit.
	Qps int

	// AllowInsecure is an option to allow insecure TLS/SSL certificates.
	AllowInsecure bool

	// DisableCompression is an option to disable compression in response
	DisableCompression bool

	// DisableKeepAlives is an option to prevents re-use of TCP connections between different HTTP requests
	DisableKeepAlives bool

	// Output represents the output type. If "csv" is provided, the
	// output will be dumped as a csv stream.
	Output string

	// ProxyAddr is the address of HTTP proxy server in the format on "host:port".
	// Optional.
	ProxyAddr *url.URL

	// ReadAll determines whether the body of the response needs
	// to be fully consumed.
	ReadAll bool

	bar     *pb.ProgressBar
	results chan *result
	stop    chan struct{}
}

func (b *Boomer) startProgress() {
	if b.Output != "" {
		return
	}
	b.bar = pb.New(b.N)
	b.bar.Format("Bom !")
	b.bar.BarStart = "Pl"
	b.bar.BarEnd = "!"
	b.bar.Empty = " "
	b.bar.Current = "a"
	b.bar.CurrentN = "a"
	b.bar.Start()
}

func (b *Boomer) finalizeProgress() {
	if b.Output != "" {
		return
	}
	b.bar.Finish()
}

func (b *Boomer) incProgress() {
	if b.Output != "" {
		return
	}
	b.bar.Increment()
}

// Run makes all the requests, prints the summary. It blocks until
// all work is done.
func (b *Boomer) Run() {
	b.results = make(chan *result, b.N)
	b.stop = make(chan struct{})
	b.startProgress()

	start := time.Now()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		b.finalizeProgress()
		close(b.stop)
	}()

	b.runWorkers()
	b.finalizeProgress()
	newReport(b.N, b.results, b.Output, time.Now().Sub(start)).finalize()
	close(b.results)
}

func (b *Boomer) runWorker(wg *sync.WaitGroup, ch chan *fasthttp.Request) {
	client := &fasthttp.Client{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: b.AllowInsecure,
		},
		MaxConnsPerHost: 65000,
	}
	resp := fasthttp.AcquireResponse()
	for req := range ch {
		s := time.Now()

		var code int
		var size int

		err := client.Do(req, resp)
		if err == nil {
			size = resp.Header.ContentLength()
			code = resp.Header.StatusCode()
		}

		b.incProgress()
		b.results <- &result{
			statusCode:    code,
			duration:      time.Now().Sub(s),
			err:           err,
			contentLength: size,
		}
	}
	wg.Done()
}

func (b *Boomer) runWorkers() {
	var wg sync.WaitGroup
	wg.Add(b.C)

	var throttle <-chan time.Time
	if b.Qps > 0 {
		throttle = time.Tick(time.Duration(1e6/(b.Qps)) * time.Microsecond)
	}

	jobsch := make(chan *fasthttp.Request, b.C)
	for i := 0; i < b.C; i++ {
		go b.runWorker(&wg, jobsch)
	}

Loop:
	for i := 0; i < b.N; i++ {
		if b.Qps > 0 {
			<-throttle
		}
		select {
		case <-b.stop:
			break Loop
		case jobsch <- cloneRequest(b.Request):
			continue
		}
	}
	close(jobsch)
	wg.Wait()
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *fasthttp.Request) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	r.CopyTo(req)
	return req
}
