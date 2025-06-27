package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
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

	webSocketUpgrader := websocket.Upgrader{} // use default options

	webSocketHandler := func(w http.ResponseWriter, r *http.Request) {
		logger := logging.GetFromContext(r.Context())

		clientConnection, err := webSocketUpgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("failed to upgrade ws connection", "err", err.Error())
			return
		}
		defer clientConnection.Close()

		for {
			msgType, message, err := clientConnection.ReadMessage()
			if err != nil {
				logger.Error("failed to read ws message", "err", err.Error())
				break
			}

			logger.Info("received ws message", "msg", string(message))

			err = clientConnection.WriteMessage(msgType, message)
			if err != nil {
				logger.Error("failed to write ws message", "err", err.Error())
				break
			}
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

			if strings.HasPrefix(r.URL.Path, "/api/live") {
				reqb, _ := httputil.DumpRequest(r, true)
				logging.GetFromContext(r.Context()).Info("handling ws request", "request", string(reqb))
				webSocketHandler(w, r)
			} else {
				proxyHandler(w, r)
			}
		})
	}
}
