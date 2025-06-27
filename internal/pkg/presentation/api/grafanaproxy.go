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

		wsURL := "ws" + grafanaURL[strings.Index(grafanaURL, ":"):] + r.URL.Path
		wsHeaders := http.Header{}

		for _, hdr := range []string{
			"Accept-Encoding", "Accept-Language", "Cookie", "Origin", "User-Agent",
			//"X-JWT-Assertion",
			"X-Real-IP", "X-Forwarded-For",
		} {
			if value, ok := r.Header[hdr]; ok {
				wsHeaders[hdr] = value
			}
		}

		logger.Info("connecting to ws endpoint", "url", wsURL, "headers", wsHeaders)
		grafanaConnection, response, err := websocket.DefaultDialer.Dial(wsURL, wsHeaders)
		if err != nil {
			logger.Error("failed to connect to grafana instance", "url", wsURL, "err", err.Error())
			if response != nil {
				respb, _ := httputil.DumpResponse(response, true)
				logger.Error("bad response", "response", string(respb))
			}
			return
		}
		defer grafanaConnection.Close()

		type msg struct {
			Type int
			Data []byte
		}

		messages := func(c *websocket.Conn) <-chan msg {
			ch := make(chan msg, 32)
			go func() {
				defer close(ch)

				for {
					msgType, payload, err := c.ReadMessage()
					if err != nil {
						return
					}

					ch <- msg{Type: msgType, Data: payload}
				}
			}()
			return ch
		}

		clientMessages := messages(clientConnection)
		grafanaMessages := messages(grafanaConnection)

		for {
			select {
			case clientMessage, ok := <-clientMessages:
				if !ok {
					return
				}

				logger.Debug("ws: client -> grafana", "payload", string(clientMessage.Data))

				grafanaConnection.WriteMessage(clientMessage.Type, clientMessage.Data)
			case grafanaMessage, ok := <-grafanaMessages:
				if !ok {
					return
				}

				logger.Debug("ws: grafana -> client", "payload", string(grafanaMessage.Data))

				clientConnection.WriteMessage(grafanaMessage.Type, grafanaMessage.Data)
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

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				r.Header.Set("X-JWT-Assertion", authHeader)
			}

			if strings.HasPrefix(r.URL.Path, "/api/live") {
				webSocketHandler(w, r)
			} else {
				proxyHandler(w, r)
			}
		})
	}
}
