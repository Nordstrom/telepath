package middleware

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func okHandlerFunc(ctx *fasthttp.RequestCtx) {
}

func Test_auth(t *testing.T) {
	cases := []struct {
		label         string
		user          string
		password      string
		authorization string
		querystring   string
		status        int
	}{
		{
			label:         "authorized-with-basic-auth",
			user:          "joe",
			password:      "secret",
			authorization: "Basic am9lOnNlY3JldA==",
			status:        http.StatusOK,
		},
		{
			label:         "unauthorized-bad-basic-auth",
			user:          "joe",
			password:      "secret",
			authorization: "Basic bm90OmEgY2hhbmNl",
			status:        http.StatusUnauthorized,
		},
		{
			label:       "authorized-with-querystring",
			user:        "joe",
			password:    "secret",
			querystring: "?u=joe&p=secret",
			status:      http.StatusOK,
		},
		{
			label:       "unauthorized-with-bad-querystring",
			user:        "joe",
			password:    "secret",
			querystring: "?u=not&p=a+chance",
			status:      http.StatusUnauthorized,
		},
		// If you authenticate with both Basic Authentication and the URL query parameters,
		// the user credentials specified in the query parameters take precedence.
		{
			label:         "authorized-with-querystring-precedence",
			user:          "joe",
			password:      "secret",
			querystring:   "?u=joe&p=secret",
			authorization: "Basic bm90OmEgY2hhbmNl",
			status:        http.StatusOK,
		},
		{
			label:         "unauthorized-with-querystring-precedence",
			user:          "joe",
			password:      "secret",
			querystring:   "?u=not&p=a+chance",
			authorization: "Basic am9lOnNlY3JldA==",
			status:        http.StatusUnauthorized,
		},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			config := AuthConfig{Enabled: true, Username: c.user, Password: c.password}
			client, teardown := newClient(Auth(okHandlerFunc, &config))
			defer teardown()

			var req fasthttp.Request
			var resp fasthttp.Response

			req.SetRequestURI("http://foo/ok" + c.querystring)
			if c.authorization != "" {
				req.Header.Add("Authorization", c.authorization)
			}
			err := client.Do(&req, &resp)

			assert.NoError(t, err)
			assert.Equal(t, c.status, resp.StatusCode())
		})
	}
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
