package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func ctxWithToken() context.Context {
	return auth.WithToken(context.Background(), "test-token")
}

func TestValidatePathID(t *testing.T) {
	is := is.New(t)
	is.NoErr(ValidatePathID("bin-1", "thing"))
	is.NoErr(ValidatePathID("11111111-1111-1111-1111-111111111111", "model"))
	is.True(ValidatePathID("", "thing") != nil)
	is.True(ValidatePathID("a/b", "thing") != nil)
	is.True(errors.Is(ValidatePathID("", "thing"), ErrBadRequest))
}

func TestBackendErrorUnwrap(t *testing.T) {
	is := is.New(t)
	is.True(errors.Is(&BackendError{Status: 400, Message: "x"}, ErrBadRequest))
	is.True(errors.Is(&BackendError{Status: 422, Message: "x"}, ErrUnprocessable))
	is.True(errors.Is(&BackendError{Status: 404}, ErrNotFound))
	is.True(errors.Is(&BackendError{Status: 409}, ErrConflict))
	is.True(errors.Is(&BackendError{Status: 401}, ErrUnauthorized))
	is.True(errors.Is(&BackendError{Status: 403}, ErrForbidden))
	is.True(strings.Contains((&BackendError{Status: 400, Message: "bad template"}).Error(), "bad template"))
}

func TestGetRawValidationErrorPreservesDetails(t *testing.T) {
	is := is.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "template unknown"})
	}))
	defer srv.Close()
	c := &Client{}
	_, _, err := c.GetRaw(ctxWithToken(), srv.URL, "things/x", url.Values{})
	var be *BackendError
	is.True(errors.As(err, &be))
	is.Equal(400, be.Status)
	is.True(strings.Contains(be.Message, "template"))
	is.True(errors.Is(err, ErrBadRequest))
}

func TestGetRawUnprocessable(t *testing.T) {
	is := is.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "semantic"})
	}))
	defer srv.Close()
	c := &Client{}
	_, _, err := c.GetRaw(ctxWithToken(), srv.URL, "things/x", url.Values{})
	is.True(errors.Is(err, ErrUnprocessable))
}

func TestWriteRawConflictAndValidation(t *testing.T) {
	is := is.New(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/conflict", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusConflict) })
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := &Client{}
	_, _, err := c.WriteRaw(ctxWithToken(), http.MethodPut, srv.URL, "conflict", url.Values{}, map[string]string{"If-Match": `"rev-1"`}, nil)
	is.True(errors.Is(err, ErrConflict))
	_, _, err = c.WriteRaw(ctxWithToken(), http.MethodPost, srv.URL, "bad", url.Values{}, nil, []byte(`{}`))
	is.True(errors.Is(err, ErrBadRequest))
}

func TestResponseBodyLimit(t *testing.T) {
	is := is.New(t)
	big := strings.Repeat("x", maxResponseBodyBytes+100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"` + big + `"`))
	}))
	defer srv.Close()
	c := &Client{}
	_, _, err := c.GetRaw(ctxWithToken(), srv.URL, "x", url.Values{})
	is.True(err != nil)
}
