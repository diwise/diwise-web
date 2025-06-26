package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func GrafanaProxy(grafanaURL string) func(http.Handler) http.Handler {
	remote, err := url.Parse(grafanaURL)
	if err != nil {
		panic(err)
	}

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Host = remote.Host
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				r.Header.Set("X-JWT-Assertion", authHeader)
			}
			p.ServeHTTP(w, r)
		}
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	return func(next http.Handler) http.Handler {
		proxyHandler := handler(proxy)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if !strings.HasPrefix(r.URL.Path, "/grafana/") {
				next.ServeHTTP(w, r)
				return
			}

			r.URL.Path = r.URL.Path[8:]

			proxyHandler(w, r)
		})
	}
}
