package middleware

import (
	"bytes"
	"encoding/base64"

	"github.com/valyala/fasthttp"
)

type AuthConfig struct {
	Enabled  bool
	Username string
	Password string
}

var basicAuthHeaderPrefix = []byte("Basic ")

// Auth is an authentication handler
func Auth(h fasthttp.RequestHandler, config *AuthConfig) fasthttp.RequestHandler {
	if !config.Enabled {
		return fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
			h(ctx)
			return
		})
	}

	return fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		username, password := readBasicAuth(ctx)

		if username == config.Username && password == config.Password {
			h(ctx)
			return
		}

		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusUnauthorized), fasthttp.StatusUnauthorized)
		ctx.Response.Header.Set("WWW-Authenticate", "Basic realm=Restricted")
	})
}

// readBasicAuth returns the username and password provided in the request's
// Authorization header, if the request uses HTTP Basic Authentication.
// See RFC 2617, Section 2.
func readBasicAuth(ctx *fasthttp.RequestCtx) (username, password string) {
	auth := ctx.Request.Header.Peek("Authorization")
	if !bytes.HasPrefix(auth, basicAuthHeaderPrefix) {
		return
	}

	payload, err := base64.StdEncoding.DecodeString(string(auth[len(basicAuthHeaderPrefix):]))
	if err != nil {
		return
	}

	pair := bytes.SplitN(payload, []byte(":"), 2)
	if len(pair) != 2 {
		return
	}

	username = string(pair[0])
	password = string(pair[1])
	return
}
