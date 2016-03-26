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

package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"encoding/base64"

	"github.com/sschepens/pb"
	"github.com/sschepens/pla/boomer"
	"github.com/sschepens/pla/reporters"
	"github.com/valyala/fasthttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	headerRegexp = `^([\w-]+):\s*(.+)`
	authRegexp   = `^(.+):([^\s].+)`
)

var (
	app      = kingpin.New("pla", "Tiny and powerful HTTP load generator.")
	n        = app.Flag("amount", "Number of requests to run.").Short('n').Default("0").Uint()
	duration = app.Flag("length", "Length or duration of test, ex: 10s, 1m, 1h, etc. Invalidates n.").Short('l').Default("0s").Duration()
	c        = app.Flag("concurrency", "Concurrency, number of requests to run concurrently. Cannot be larger than n.").Short('c').Default("0").Uint()
	q        = app.Flag("qps", "Rate Limit, in seconds (QPS).").Short('q').Default("0").Uint()

	m          = app.Flag("method", "HTTP method.").Short('m').Default("GET").String()
	headerList = app.Flag("header", "Add custom HTTP header, name1:value1. Can be repeated for more headers.").Short('H').Strings()
	timeout    = app.Flag("timeout", "Request timeout, ex: 10s, 1m, 1h, etc.").Short('t').Default("0s").Duration()
	body       = app.Flag("body", "Request Body.").Short('d').Default("").String()
	authHeader = app.Flag("auth", "Basic Authentication, username:password.").Short('a').Default("").String()

	disableCompression = app.Flag("disable-compression", "Disable compression.").Default("false").Bool()
	disableKeepAlives  = app.Flag("disable-keepalive", "Disable keep-alive.").Default("false").Bool()

	url            = app.Arg("url", "Request URL").Required().String()
	boomerInstance *boomer.Boomer
	progressBar    *pb.ProgressBar
	reporter       *reporters.StaticReport
)

func main() {
	app.HelpFlag.Short('h')
	if len(os.Args) < 2 {
		usageAndExit("")
	}
	_, err := app.Parse(os.Args[1:])
	if err != nil {
		usageAndExit(err.Error())
	}

	if *duration <= 0 && *n <= 0 {
		usageAndExit("length or amount must be specified")
	}

	if *c <= 0 {
		usageAndExit("cconcurrency cannot be smaller than 1.")
	}

	if *n > 0 && *c > *n {
		usageAndExit("concurrency cannot be greater than amount")
	}

	var (
		method string
		// Username and password for basic auth
		username, password string
	)

	method = strings.ToUpper(*m)

	// set basic auth if set
	if *authHeader != "" {
		match, err := parseInputWithRegexp(*authHeader, authRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		username, password = match[1], match[2]
	}

	req := fasthttp.AcquireRequest()
	req.URI().Update(*url)
	if len(req.URI().Host()) == 0 {
		req.URI().Update("http://" + *url)
		if len(req.URI().Host()) == 0 {
			usageAndExit("invalid url ''" + req.URI().String() + "'', unable to detect host")
		}
	}
	req.Header.SetMethod(method)
	req.SetBodyString(*body)
	req.Header.SetContentLength(len(req.Body()))
	if username != "" || password != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	// set any other additional headers
	for _, h := range *headerList {
		match, err := parseInputWithRegexp(h, headerRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		req.Header.Set(match[1], match[2])
	}

	if !*disableCompression {
		req.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	if *disableKeepAlives {
		req.SetConnectionClose()
	}

	reporter = reporters.NewStaticReport()
	progressBar = newProgressBar()
	boomerInstance = boomer.NewBoomer(req).
		WithAmount(*n).
		WithConcurrency(*c).
		WithDuration(*duration).
		WithTimeout(*timeout).
		WithRateLimit(*q, time.Second)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		boomerInstance.Stop()
	}()

	boomerInstance.Run()
	go processResults()
	boomerInstance.Wait()
	time.Sleep(1 * time.Millisecond)
	progressBar.Finish()
	reporter.Finalize()
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, "Error: %s", msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	app.Usage(os.Args[1:])
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func parseInputWithRegexp(input, regx string) ([]string, error) {
	re := regexp.MustCompile(regx)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 1 {
		return nil, fmt.Errorf("could not parse the provided input; input = %v", input)
	}
	return matches, nil
}

func newProgressBar() *pb.ProgressBar {
	if *duration > 0 {
		progressBar = pb.New(100)
		ticker := time.NewTicker(*duration / 100)
		go func() {
			for range ticker.C {
				progressBar.Increment()
			}
		}()
	} else {
		progressBar = pb.New(int(*n))
	}
	progressBar.BarStart = "Pl"
	progressBar.BarEnd = "!"
	progressBar.Empty = " "
	progressBar.Current = "a"
	progressBar.CurrentN = "a"
	progressBar.Start()
	return progressBar
}

func processResults() {
	for res := range boomerInstance.Results() {
		reporter.ProcessResult(res)
		if *duration == 0 {
			progressBar.Increment()
		}
	}
}
