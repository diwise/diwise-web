package api

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/diwise/diwise-web/internal/presentation/api/authz"
)

func AuthzDeniedResponse(deniedHandler authz.DeniedHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if deniedHandler == nil || isStreamingOrUpgradeRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			aw := newAuthzDeniedResponseWriter(w)
			next.ServeHTTP(aw, r)

			if aw.denialStatus == 0 {
				return
			}

			reason := authz.DenialReasonForbidden
			if aw.denialStatus == http.StatusUnauthorized {
				reason = authz.DenialReasonUnauthenticated
			}

			deniedHandler(w, r, authz.Denial{
				Status: aw.denialStatus,
				Reason: reason,
			})
		})
	}
}

type authzDeniedResponseWriter struct {
	rw           http.ResponseWriter
	header       http.Header
	denialStatus int
	wroteHeader  bool
}

func newAuthzDeniedResponseWriter(rw http.ResponseWriter) *authzDeniedResponseWriter {
	return &authzDeniedResponseWriter{
		rw:     rw,
		header: make(http.Header),
	}
}

func (w *authzDeniedResponseWriter) Header() http.Header {
	return w.header
}

func (w *authzDeniedResponseWriter) Write(data []byte) (int, error) {
	if w.denialStatus != 0 {
		return len(data), nil
	}

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.rw.Write(data)
}

func (w *authzDeniedResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}

	w.wroteHeader = true

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		w.denialStatus = statusCode
		return
	}

	copyHeaders(w.rw.Header(), w.header)
	w.rw.WriteHeader(statusCode)
}

func (w *authzDeniedResponseWriter) Flush() {
	if w.denialStatus != 0 {
		return
	}

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if f, ok := w.rw.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *authzDeniedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.rw.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("authz denied response writer does not implement http.Hijacker")
	}

	return h.Hijack()
}

func (w *authzDeniedResponseWriter) Unwrap() http.ResponseWriter {
	return w.rw
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}

func isStreamingOrUpgradeRequest(r *http.Request) bool {
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return true
	}

	if strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return true
	}

	return strings.TrimSpace(r.Header.Get("Upgrade")) != ""
}

//Important reason for the private header map: http.Error sets headers before it calls WriteHeader. If the wrapper returned the real headers immediately, text/plain and other error headers could leak
//into the final redirect/toast response. This wrapper keeps those headers isolated unless the status is not 401/403.
