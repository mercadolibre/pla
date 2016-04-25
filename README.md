# pla [![Build Status](https://travis-ci.org/sschepens/pla.svg?branch=master)](https://travis-ci.org/sschepens/pla)

Pla is a tiny program that sends some load to a web application. It's similar to Apache Bench ([ab](http://httpd.apache.org/docs/2.2/programs/ab.html)), but with better availability across different platforms and a less troubling installation experience.

Pla is originally written by [rakyll](https://github.com/rakyll) and is available at [rakyll/boom](https://github.com/rakyll/boom).

Due to some issues like memory consumption I decided to fork it.

## Installation

Simple as it takes to type the following command:

    go get -u github.com/sschepens/pla

## Usage

Pla supports custom headers, request body and basic authentication. It runs provided number of requests in the provided concurrency level, and prints stats.
~~~
usage: pla [<flags>] <url>

Tiny and powerful HTTP load generator.

Flags:
  -h, --help                 Show context-sensitive help (also try --help-long and --help-man).
  -n, --amount=0             Number of requests to run.
  -l, --length=0s            Length or duration of test, ex: 10s, 1m, 1h, etc. Invalidates n.
  -c, --concurrency=0        Concurrency, number of requests to run concurrently. If concurrency is set as 0 pla will run with the same amount of cores that the processor has.
                             Cannot be larger than n.
  -q, --qps=0                Rate Limit, in seconds (QPS).
  -m, --method="GET"         HTTP method.
  -H, --header=HEADER ...    Add custom HTTP header, name1:value1. Can be repeated for more headers.
  -t, --timeout=0s           Request timeout, ex: 10s, 1m, 1h, etc.
  -d, --body=""              Request Body.
  -a, --auth=""              Basic Authentication, username:password.
      --disable-compression  Disable compression.
      --disable-keepalive    Disable keep-alive.

Args:
  <url>  Request URL
~~~

This is what happens when you run Pla:

	% pla -n 1000 -c 100 https://google.com
	1000 / 1000 ∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎ 100.00 %

	Summary:
	  Total:        21.1307 secs.
	  Slowest:      2.9959 secs.
	  Fastest:      0.9868 secs.
	  Average:      2.0827 secs.
	  Requests/sec: 47.3246
	  Speed index:  Hahahaha

	Response time histogram:
      0.987 [1]     |
      1.188 [2]     |
      1.389 [3]     |
      1.590 [18]    |∎∎
      1.790 [85]    |∎∎∎∎∎∎∎∎∎∎∎
      1.991 [244]   |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
      2.192 [284]   |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
      2.393 [304]   |∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎∎
      2.594 [50]    |∎∎∎∎∎∎
      2.795 [5]     |
      2.996 [4]     |

	Latency distribution:
	  10% in 1.7607 secs.
	  25% in 1.9770 secs.
	  50% in 2.0961 secs.
	  75% in 2.2385 secs.
	  90% in 2.3681 secs.
	  95% in 2.4451 secs.
	  99% in 2.5393 secs.

	Status code distribution:
	  [200]	1000 responses

## License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
