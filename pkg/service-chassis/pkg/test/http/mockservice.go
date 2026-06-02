package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"github.com/matryer/is"
)

func NewMockServiceThat(expects func(r *http.Request), returns func(w http.ResponseWriter)) MockService {

	mock := &mockSvc{
		requestCount: &atomic.Int32{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.requestCount.Add(1)
		expects(r)
		returns(w)
	}))

	mock.server = srv

	return mock
}

type MockService interface {
	Close()
	RequestCount() int
	URL() string
}

type mockSvc struct {
	requestCount *atomic.Int32
	server       *httptest.Server
	closed       bool
}

func (m *mockSvc) Close() {
	if !m.closed {
		m.server.Close()
	}
	m.closed = true
}

func (m *mockSvc) RequestCount() int {
	return int(m.requestCount.Load())
}

func (m *mockSvc) URL() string {
	return m.server.URL
}

func Expects(is *is.I, facts ...func(*is.I, *http.Request)) func(r *http.Request) {
	return func(r *http.Request) {
		for _, checkFact := range facts {
			checkFact(is, r)
		}
	}
}

func Returns(writers ...func(w http.ResponseWriter)) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		for _, writeResult := range writers {
			writeResult(w)
		}
	}
}
