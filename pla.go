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

	"github.com/sschepens/pla/boomer"
	"github.com/sschepens/pla/interfaces"
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
	c        = app.Flag("concurrency", "Concurrency, number of requests to run concurrently. If concurrency is set as 0 pla will run with the same amount of cores that the processor has. Cannot be larger than n.").Short('c').Default("0").Uint()
	q        = app.Flag("qps", "Rate Limit, in seconds (QPS).").Short('q').Default("0").Uint()
	f        = app.Flag("fail", "Abort on request failure.").Short('f').Default("false").Bool()

	m          = app.Flag("method", "HTTP method.").Short('m').Default("GET").String()
	headerList = app.Flag("header", "Add custom HTTP header, name1:value1. Can be repeated for more headers.").Short('H').Strings()
	body       = app.Flag("body", "Request Body.").Short('d').Default("").String()
	authHeader = app.Flag("auth", "Basic Authentication, username:password.").Short('a').Default("").String()

	timeout            = app.Flag("timeout", "Timeout for the hole request connect+write+read, ex: 10s, 1m, 1h, etc.").Short('t').Default("30s").Duration()
	connectTimeout     = app.Flag("connect-timeout", "Connect timeout, ex: 10s, 1m, 1h, etc.").Default("5s").Duration()
	readTimeout        = app.Flag("read-timeout", "Request read timeout, ex: 10s, 1m, 1h, etc.").Default("0s").Duration()
	writeTimeout       = app.Flag("write-timeout", "Request write timeout, ex: 10s, 1m, 1h, etc.").Default("0s").Duration()
	disableCompression = app.Flag("disable-compression", "Disable compression.").Default("false").Bool()
	disableKeepAlives  = app.Flag("disable-keepalive", "Disable keep-alive.").Default("false").Bool()

	url            = app.Arg("url", "Request URL").Required().String()
	boomerInstance *boomer.Boomer
	ui             Interface
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

	if *c < 0 {
		usageAndExit("concurrency cannot be smaller than 0")
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

	ui = interfaces.NewBasicInterface()
	boomerInstance = boomer.NewBoomer(req).
		WithAmount(*n).
		WithConcurrency(*c).
		WithDuration(*duration).
		WithTimeout(*timeout).
		WithRateLimit(*q, time.Second).
		WithAbortionOnFailure(*f)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		boomerInstance.Stop()
		ui.End()
		os.Exit(1)
	}()

	ui.Start(boomerInstance)
	boomerInstance.Run()
	go processResults()
	boomerInstance.Wait()
	time.Sleep(1 * time.Millisecond)
	ui.End()
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

func processResults() {
	for res := range boomerInstance.Results() {
		ui.ProcessResult(res)
	}
}
