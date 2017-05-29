package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_ping_handler(t *testing.T) {
	client, teardown := newClient(pingHandlerFunc)
	defer teardown()

	statusCode, _, err := client.Get(nil, "http://foo/ping")
	assert.NoError(t, err)
	assert.Equal(t, 204, statusCode)
}

func Test_query_handler(t *testing.T) {
	client, teardown := newClient(queryHandlerFunc)
	defer teardown()

	var req fasthttp.Request
	var resp fasthttp.Response

	req.SetRequestURI("http://foo/query")
	err := client.Do(&req, &resp)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode())
	assert.Equal(t, "application/json", string(resp.Header.Peek("Content-Type")))
	assert.Equal(t, "1.0", string(resp.Header.Peek("X-Influxdb-Version")))
	assert.Equal(t, `{"results":[]}`, string(resp.Body()))
}

func Test_write_handler_verbs(t *testing.T) {
	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	client, teardown := newClient(makeWriteHandler(p, writeConfig{}))
	defer teardown()

	verbs := []string{"GET", "PUT", "FOO"}
	for _, verb := range verbs {
		var req fasthttp.Request
		var resp fasthttp.Response

		req.SetRequestURI("http://foo/write?db=test")
		req.Header.SetMethod(verb)

		client.Do(&req, &resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
	}
}

func Test_write_handler_without_db_param(t *testing.T) {
	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	client, teardown := newClient(makeWriteHandler(p, writeConfig{}))
	defer teardown()

	statusCode, _, err := client.Post(nil, "http://foo/write", nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, statusCode)
}

func Test_write_handler_with_empty_payload(t *testing.T) {
	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	client, teardown := newClient(makeWriteHandler(p, writeConfig{}))
	defer teardown()

	statusCode, _, err := client.Post(nil, "http://foo/write?db=test", nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, statusCode)
}

func Test_write_handler_with_metrics(t *testing.T) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	cases := []struct {
		url      string
		encoding string
		body     []byte
		lines    []string
	}{
		{
			url:  "http://foo/write?db=test",
			body: []byte("simple_metric,x=y value=1 1494462271\n"),
			lines: []string{
				"simple_metric,x=y value=1 1494462271",
			},
		},
		{
			url:      "http://foo/write?db=test",
			encoding: "gzip",
			body:     makeGzipString("gzipped_metric,x=y value=1 1494462271\n"),
			lines: []string{
				"gzipped_metric,x=y value=1 1494462271",
			},
		},
		{
			url:  "http://foo/write?db=test",
			body: []byte("no_trailing_newline_metric,x=y value=1 1494462271"),
			lines: []string{
				"no_trailing_newline_metric,x=y value=1 1494462271",
			},
		},
		{
			url:  "http://foo/write?db=test",
			body: []byte("multiple_metrics,x=y value=1 1494462271\nmultiple_metrics,x=z value=1 1494462271\n"),
			lines: []string{
				"multiple_metrics,x=y value=1 1494462271",
				"multiple_metrics,x=z value=1 1494462271",
			},
		},
		{
			url:  "http://foo/write?db=test&precision=s",
			body: []byte("upscale_from_seconds_metric,x=y value=1 6494462272\n"),
			lines: []string{
				"upscale_from_seconds_metric,x=y value=1 6494462272000000000",
			},
		},
	}

	for _, c := range cases {
		p := mocks.NewAsyncProducer(t, config)
		defer p.Close()

		client, teardown := newClient(makeWriteHandler(p, writeConfig{}))
		defer teardown()

		for _ = range c.lines {
			p.ExpectInputAndSucceed()
		}

		var req fasthttp.Request
		var resp fasthttp.Response

		req.SetRequestURI(c.url)
		req.Header.SetMethod("POST")
		req.Header.Add("Content-Encoding", c.encoding)
		req.SetBody(c.body)
		err := client.Do(&req, &resp)

		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode())

		for _, line := range c.lines {
			select {
			case msg := <-p.Successes():
				metric, _ := msg.Value.Encode()
				assert.Equal(t, line, string(metric))
			case <-time.After(time.Second):
				t.Fatalf("Timeout while waiting for message from channel")
			}
		}
	}
}

func Test_write_handler_with_oversized_payload(t *testing.T) {
	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	client, teardown := newClient(makeWriteHandler(p, writeConfig{maxBodySize: 1024}))
	defer teardown()

	var req fasthttp.Request
	var resp fasthttp.Response

	req.SetRequestURI("http://foo/write?db=test")
	req.Header.SetMethod("POST")
	req.Header.Add("Content-Encoding", "text/plain")
	req.SetBody([]byte(make([]byte, 1024+1)))
	err := client.Do(&req, &resp)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode())
}

func Test_write_handler_with_oversized_metric_line(t *testing.T) {
	t.Skip("Not implemented")
	//p := mocks.NewAsyncProducer(t, nil)
	//defer p.Close()

	//client, teardown := newClient(makeWriteHandler(p, writeConfig{maxLineSize: 32}))
	//defer teardown()

	//var req fasthttp.Request
	//var resp fasthttp.Response

	//req.SetRequestURI("http://foo/write?db=test")
	//req.Header.SetMethod("POST")
	//req.Header.Add("Content-Encoding", "text/plain")
	//req.SetBody([]byte("foo,x=y very=1 long=2 metric=3 1494462271"))
	//err := client.Do(&req, &resp)

	//assert.NoError(t, err)
	//assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
}

func Test_write_handler_with_broken_gzip_payload(t *testing.T) {
	p := mocks.NewAsyncProducer(t, nil)
	defer p.Close()

	client, teardown := newClient(makeWriteHandler(p, writeConfig{}))
	defer teardown()

	var req fasthttp.Request
	var resp fasthttp.Response

	req.SetRequestURI("http://foo/write?db=test")
	req.Header.SetMethod("POST")
	req.Header.Add("Content-Encoding", "gzip")
	req.SetBody([]byte("bogus"))
	err := client.Do(&req, &resp)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
}

func newClient(handlerFunc func(*fasthttp.RequestCtx)) (*fasthttp.Client, func()) {
	server := &fasthttp.Server{
		Handler: handlerFunc,
		DisableHeaderNamesNormalizing: true,
	}

	listener := fasthttputil.NewInmemoryListener()
	go func() {
		if err := server.Serve(listener); err != nil {
			fmt.Printf("Unexpected error: %s", err)
		}
	}()

	return &fasthttp.Client{
			DisableHeaderNamesNormalizing: true,
			Dial: func(addr string) (net.Conn, error) {
				return listener.Dial()
			},
		}, func() {
			listener.Close()
		}
}

func makeWriteHandler(producer sarama.AsyncProducer, config writeConfig) func(*fasthttp.RequestCtx) {
	wh, err := NewWriteHandler(producer, config)
	if err != nil {
		panic(err)
	}

	return wh.Handle
}

func makeGzipString(str string) []byte {
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
