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
	"flag"
	"fmt"
	gourl "net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"encoding/base64"
	"github.com/sschepens/pla/boomer"
	"github.com/valyala/fasthttp"
)

const (
	headerRegexp = `^([\w-]+):\s*(.+)`
	authRegexp   = `^(.+):([^\s].+)`
)

type stringSlice []string

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

var (
	headerList  stringSlice
	m           = flag.String("m", "GET", "")
	headers     = flag.String("h", "", "")
	body        = flag.String("d", "", "")
	accept      = flag.String("A", "", "")
	contentType = flag.String("T", "text/html", "")
	authHeader  = flag.String("a", "", "")
	readAll     = flag.Bool("readall", false, "")

	output = flag.String("o", "", "")

	c    = flag.Int("c", 50, "")
	n    = flag.Int("n", 200, "")
	q    = flag.Int("q", 0, "")
	t    = flag.Int("t", 0, "")
	duration = flag.Int("l", 0, "")
	cpus = flag.Int("cpus", runtime.GOMAXPROCS(-1), "")

	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	proxyAddr          = flag.String("x", "", "")
)

var usage = `Usage: pla [options...] <url>

Options:
  -n  Number of requests to run.
  -c  Number of requests to run concurrently. Total number of requests cannot
      be smaller than the concurency level.
  -q  Rate limit, in seconds (QPS).
  -o  Output type. If none provided, a summary is printed.
      "csv" is the only supported alternative. Dumps the response
      metrics in comma-seperated values format.

  -m  HTTP method, one of GET, POST, PUT, DELETE, HEAD, OPTIONS.
  -H  Add custom HTTP header, name1:value1. Can be repeated for more headers.
  -h  Custom HTTP headers, name1:value1;name2:value2.
  -t  Timeout in ms.
  -A  HTTP Accept header.
  -d  HTTP request body.
	-l  Length or duration of test in minutes, invalidates number of requests.
  -T  Content-type, defaults to "text/html".
  -a  Basic authentication, username:password.
  -x  HTTP Proxy address as host:port.

  -readall              Consumes the entire request body.
  -disable-compression  Disable compression.
  -disable-keepalive    Disable keep-alive, prevents re-use of TCP
                        connections between different HTTP requests.
  -cpus                 Number of used cpu cores.
                        (default for current machine is %d cores)
`

func main() {
	flag.Var(&headerList, "H", "")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}

	flag.Parse()
	if flag.NArg() < 1 {
		usageAndExit("")
	}

	runtime.GOMAXPROCS(*cpus)
	num := *n
	conc := *c
	q := *q

	if num <= 0 || conc <= 0 {
		usageAndExit("n and c cannot be smaller than 1.")
	}

	if *duration < 0 {
		usageAndExit("duration cannot be negative.")
	}

	var (
		url, method string
		// Username and password for basic auth
		username, password string
		// request headers
	)

	url = flag.Args()[0]
	method = strings.ToUpper(*m)

	if *output != "csv" && *output != "" {
		usageAndExit("Invalid output type; only csv is supported.")
	}

	var proxyURL *gourl.URL
	if *proxyAddr != "" {
		var err error
		proxyURL, err = gourl.Parse(*proxyAddr)
		if err != nil {
			usageAndExit(err.Error())
		}
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	req.Header.SetMethod(method)
	req.SetBodyString(*body)
	if username != "" || password != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
	}

	// set content-type
	req.Header.Set("Content-Type", *contentType)
	// set any other additional headers
	if *headers != "" {
		headers := strings.Split(*headers, ";")
		for _, h := range headers {
			match, err := parseInputWithRegexp(h, headerRegexp)
			if err != nil {
				usageAndExit(err.Error())
			}
			req.Header.Set(match[1], match[2])
		}
	}
	for _, h := range headerList {
		match, err := parseInputWithRegexp(h, headerRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		req.Header.Set(match[1], match[2])
	}

	if *accept != "" {
		req.Header.Set("Accept", *accept)
	}

	if !*disableCompression {
		req.Header.Set("Accept-Encoding", "gzip,deflate")
	}

	if *disableKeepAlives {
		req.SetConnectionClose()
	}

	// set basic auth if set
	if *authHeader != "" {
		match, err := parseInputWithRegexp(*authHeader, authRegexp)
		if err != nil {
			usageAndExit(err.Error())
		}
		username, password = match[1], match[2]
	}

	(&boomer.Boomer{
		Request:       req,
		N:             num,
		C:             conc,
		Duration:      time.Duration(*duration) * time.Minute,
		QPS:           q,
		Timeout:       time.Duration(*t) * time.Millisecond,
		ProxyAddr:     proxyURL,
		Output:        *output,
		ReadAll:       *readAll,
	}).Run()
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
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
