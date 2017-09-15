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
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestN(t *testing.T) {
	var count int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, int64(1))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(server.URL)
	req.Header.SetMethod("GET")
	boomer := NewBoomer(string(req.Host()), req).
		WithAmount(20).
		WithConcurrency(2)
	go func() {
		for range boomer.Results() {
		}
	}()
	boomer.Run()
	boomer.Wait()
	if atomic.LoadInt64(&count) != 20 {
		t.Errorf("Expected to boom 20 times, found %d", atomic.LoadInt64(&count))
	}
}

func TestQPS(t *testing.T) {
	var wg sync.WaitGroup
	var count int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, int64(1))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(server.URL)
	req.Header.SetMethod("GET")
	boomer := NewBoomer(string(req.Host()), req).
		WithAmount(20).
		WithConcurrency(2).
		WithRateLimit(1, time.Second)
	go func() {
		for range boomer.Results() {
		}
	}()
	wg.Add(1)
	time.AfterFunc(time.Second, func() {
		if atomic.LoadInt64(&count) > 1 {
			t.Errorf("Expected to boom 1 times, found %d", atomic.LoadInt64(&count))
		}
		wg.Done()
	})
	boomer.Run()
	wg.Wait()
	boomer.Stop()
	boomer.Wait()
}

func TestRequest(t *testing.T) {
	var uri, contentType, some, method, auth string
	handler := func(w http.ResponseWriter, r *http.Request) {
		uri = r.RequestURI
		method = r.Method
		contentType = r.Header.Get("Content-type")
		some = r.Header.Get("X-some")
		auth = r.Header.Get("Authorization")
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(server.URL)
	req.Header.SetMethod("GET")
	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("X-some", "value")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("username:password")))
	boomer := NewBoomer(string(req.Host()), req).
		WithAmount(1).
		WithConcurrency(1)
	go func() {
		for range boomer.Results() {
		}
	}()
	boomer.Run()
	boomer.Wait()
	if uri != "/" {
		t.Errorf("Uri is expected to be /, %v is found", uri)
	}
	if contentType != "text/html" {
		t.Errorf("Content type is expected to be text/html, %v is found", contentType)
	}
	if some != "value" {
		t.Errorf("X-some header is expected to be value, %v is found", some)
	}
	if auth != "Basic dXNlcm5hbWU6cGFzc3dvcmQ=" {
		t.Errorf("Basic authorization is not properly set: " + auth)
	}
}

func TestBody(t *testing.T) {
	var count int64
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		if string(body) == "Body" {
			atomic.AddInt64(&count, 1)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(server.URL)
	req.Header.SetMethod("POST")
	req.SetBody([]byte("Body"))
	boomer := NewBoomer(string(req.Host()), req).
		WithAmount(10).
		WithConcurrency(1)
	go func() {
		for range boomer.Results() {
		}
	}()
	boomer.Run()
	boomer.Wait()
	if atomic.LoadInt64(&count) != 10 {
		t.Errorf("Expected to boom 10 times, found %d", atomic.LoadInt64(&count))
	}
}
