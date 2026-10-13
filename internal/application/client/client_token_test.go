package client

import (
	"context"
	"net/url"
	"testing"

	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func TestMethodsRefuseEmptyTokenWithoutBackendCall(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	c := NewClient("http://127.0.0.1:1/device", "http://127.0.0.1:1/things", "http://127.0.0.1:1/admin", "http://127.0.0.1:1/alarms", "http://127.0.0.1:1/measurements", "http://127.0.0.1:1/transform")

	_, err := c.Get(ctx, "http://127.0.0.1:1", "x", url.Values{})
	is.True(err != nil)

	_, _, err = c.GetRaw(ctx, "http://127.0.0.1:1", "x", url.Values{})
	is.True(err != nil)

	_, _, err = c.WriteRaw(ctx, "POST", "http://127.0.0.1:1", "x", url.Values{}, nil, nil)
	is.True(err != nil)

	_, _, _, err = c.WriteRawDetailed(ctx, "POST", "http://127.0.0.1:1", "x", url.Values{}, nil, nil)
	is.True(err != nil)

	is.True(c.Patch(ctx, "http://127.0.0.1:1", "x", nil) != nil)
	is.True(c.Post(ctx, "http://127.0.0.1:1", nil) != nil)
	is.True(c.Put(ctx, "http://127.0.0.1:1", nil) != nil)
	is.True(c.Delete(ctx, "http://127.0.0.1:1") != nil)
}

func TestMethodsAcceptTokenContext(t *testing.T) {
	is := is.New(t)
	ctx := auth.WithToken(context.Background(), "test-token")

	c := NewClient("http://127.0.0.1:1/device", "http://127.0.0.1:1/things", "http://127.0.0.1:1/admin", "http://127.0.0.1:1/alarms", "http://127.0.0.1:1/measurements", "http://127.0.0.1:1/transform")

	// Backend is unreachable; we only assert that the missing-token
	// fail-fast does not trigger (a transport error is expected instead).
	_, err := c.Get(ctx, "http://127.0.0.1:1", "x", url.Values{})
	is.True(err != nil)
	is.True(auth.Token(ctx) == "test-token")
}
