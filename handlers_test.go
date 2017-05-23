package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_ping_handler(t *testing.T) {
	var req fasthttp.Request
	var resp fasthttp.Response

	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	newHandler(listener, pingHandlerFunc)
	client := newClient(listener)

	req.SetRequestURI("http://foo/ping")
	err := client.Do(&req, &resp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode() != 204 {
		t.Errorf("Expected StatusCode 204, but was %d", resp.StatusCode())
	}
}

func Test_query_handler(t *testing.T) {
	var req fasthttp.Request
	var resp fasthttp.Response

	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	newHandler(listener, queryHandlerFunc)
	client := newClient(listener)

	req.SetRequestURI("http://foo/query")
	err := client.Do(&req, &resp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("Expected StatusCode 200, but was %d", resp.StatusCode())
	}

	if header := resp.Header.Peek("Content-Type"); string(header) != "application/json" {
		t.Errorf("Expected Header 'Content-Type: application/json', but was '%v'", string(header))
	}

	if header := resp.Header.Peek("X-Influxdb-Version"); string(header) != "1.0" {
		t.Errorf("Expected Header 'X-Influxdb-Version: 1.0', but was '%v'", string(header))
	}

	body := resp.Body()
	if string(body) != `{"results":[]}` {
		t.Errorf("Expected empty result-set; but was '%v'", string(body))
	}
}

func Test_write_handler_verb(t *testing.T) {
	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	newWriteHandler(listener, p, writeConfig{})
	client := newClient(listener)

	verbs := []string{"GET", "PUT", "FOO"}
	for _, verb := range verbs {
		var req fasthttp.Request
		var resp fasthttp.Response

		req.SetRequestURI("http://foo/write")
		req.Header.SetMethod(verb)

		client.Do(&req, &resp)
		if resp.StatusCode() != http.StatusBadRequest {
			t.Errorf("Expected StatusCode 400, but was: %d", resp.StatusCode())
		}
	}
}

func Test_write_handler_payload_too_large(t *testing.T) {
	var req fasthttp.Request
	var resp fasthttp.Response

	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	newWriteHandler(listener, p, writeConfig{
		maxBodySize: 1024,
	})
	client := newClient(listener)

	req.SetRequestURI("http://foo/write")
	req.Header.SetMethod("POST")
	req.SetBody(make([]byte, 1024+1))
	client.Do(&req, &resp)

	if resp.StatusCode() != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected StatusCode 413, but was %d", resp.StatusCode())
	}
}

func Test_write_handler_bogus_gzip_payload(t *testing.T) {
	var req fasthttp.Request
	var resp fasthttp.Response

	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	newWriteHandler(listener, p, writeConfig{})
	client := newClient(listener)

	req.SetRequestURI("http://foo/write")
	req.Header.SetMethod("POST")
	req.Header.Add("Content-Encoding", "gzip")
	req.SetBody([]byte("bogus"))
	client.Do(&req, &resp)

	if resp.StatusCode() != http.StatusBadRequest {
		t.Errorf("Expected StatusCode 400, but was %d", resp.StatusCode())
	}
}

func Test_write_handler_metrics_payload(t *testing.T) {
	chunkSize := 64
	cases := []struct {
		uri      string
		encoding string
		label    string
		body     []byte
		lines    []string
		status   int
	}{
		{
			uri:      "http://foo/write?precision=ns",
			encoding: "text/plain",
			label:    "Missing db param",
			body:     []byte{},
			lines:    []string{},
			status:   http.StatusBadRequest,
		}, {
			uri:      "http://foo/write?db=test",
			encoding: "text/plain",
			label:    "Missing precision param",
			body:     []byte{},
			lines:    []string{},
			status:   http.StatusNoContent,
		}, {
			uri:      "http://foo/write?db=test&precision=ns",
			encoding: "text/plain",
			label:    "Empty metric payload",
			body:     []byte{},
			lines:    []string{},
			status:   http.StatusNoContent,
		}, {
			uri:      "http://foo/write?db=test&precision=ns",
			encoding: "text/plain",
			label:    "Single metric",
			body:     []byte("foo,x=y value=1 1494462271\n"),
			lines: []string{
				"foo,x=y value=1 1494462271&precision=ns",
			},
			status: http.StatusNoContent,
		}, {
			uri:      "http://foo/write?db=test&precision=ns",
			encoding: "text/plain",
			label:    "Multiple metrics",
			body:     []byte("foo,x=y value=1 1494462271\nbar,x=y value=2 1494462272\n"),
			lines: []string{
				"foo,x=y value=1 1494462271",
				"bar,x=y value=2 1494462272",
			},
			status: http.StatusNoContent,
		}, {
			uri:      "http://foo/write?db=test&percision=ns",
			encoding: "gzip",
			label:    "Gzipped metrics",
			body:     gzipString("foo,x=y value=1 1494462271\nbar,x=y value=2 1494462272\n"),
			lines: []string{
				"foo,x=y value=1 1494462271",
				"bar,x=y value=2 1494462272",
			},
			status: http.StatusNoContent,
		},
	}

	listener := fasthttputil.NewInmemoryListener()
	defer listener.Close()

	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	for _, c := range cases {
		var req fasthttp.Request
		var resp fasthttp.Response

		for _ = range c.lines {
			p.ExpectInputWithCheckerFunctionAndSucceed(func(b []byte) error {
				for _, line := range c.lines {
					if line == string(b) {
						return nil
					}
				}

				return fmt.Errorf("%s: Found unexpected line '%s'", c.label, string(b))
			})
		}

		newWriteHandler(listener, p, writeConfig{
			maxChunkSize:  chunkSize,
			topicTemplate: "telepath-metrics",
		})

		client := newClient(listener)

		req.SetRequestURI(c.uri)
		req.Header.SetMethod("POST")
		req.Header.Add("Content-Encoding", c.encoding)
		req.SetBody(c.body)
		client.Do(&req, &resp)

		if resp.StatusCode() != c.status {
			t.Errorf("%s: expected StatusCode %d, but was %d", c.label, c.status, resp.StatusCode())
		}
	}
}

func newWriteHandler(listener net.Listener, producer sarama.AsyncProducer, config writeConfig) {
	wh, _ := NewWriteHandler(producer, config)
	newHandler(listener, wh.Handle)
}

func newHandler(listener net.Listener, handlerFunc func(*fasthttp.RequestCtx)) {
	server := &fasthttp.Server{
		Handler: handlerFunc,
		DisableHeaderNamesNormalizing: true,
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			fmt.Printf("Unexpected error: %s", err)
		}
	}()
}

func newClient(listener *fasthttputil.InmemoryListener) *fasthttp.Client {
	return &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return listener.Dial()
		},
		DisableHeaderNamesNormalizing: true,
	}
}

func gzipString(str string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(str)); err != nil {
		panic(err)
	}
	if err := gz.Flush(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}

	return b.Bytes()
}
