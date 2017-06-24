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
		if success, attempt := passQuerystringAuth(ctx, config.Username, config.Password); attempt {
			if success {
				h(ctx)
				return
			}
		} else if success, _ := passBasicAuth(ctx, config.Username, config.Password); success {
			h(ctx)
			return
		}

		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusUnauthorized), fasthttp.StatusUnauthorized)
		ctx.Response.Header.Set("WWW-Authenticate", "Basic realm=Restricted")
	})
}

func passQuerystringAuth(ctx *fasthttp.RequestCtx, username, password string) (success, attempt bool) {
	u := ctx.QueryArgs().Peek("u")
	p := ctx.QueryArgs().Peek("p")
	if u == nil {
		return
	}

	attempt = true
	success = string(u) == username && p != nil && string(p) == password
	return
}

func passBasicAuth(ctx *fasthttp.RequestCtx, username, password string) (success, attempt bool) {
	auth := ctx.Request.Header.Peek("Authorization")
	if !bytes.HasPrefix(auth, basicAuthHeaderPrefix) {
		return
	}

	attempt = true
	payload, err := base64.StdEncoding.DecodeString(string(auth[len(basicAuthHeaderPrefix):]))
	if err != nil {
		return
	}

	pair := bytes.SplitN(payload, []byte(":"), 2)
	if len(pair) != 2 {
		return
	}

	success = string(pair[0]) == username && string(pair[1]) == password
	return
}
