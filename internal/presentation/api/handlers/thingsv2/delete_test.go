package thingsv2

import (
	"context"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingsV2DeleteDialogRendersConfirm(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DeleteDialog(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/delete?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "thing-v2-delete-dialog"))
}

func TestThingsV2DeletePageRedirectsOnSuccess(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DeletePage(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/delete?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2DeletePageShowsErrorForUnknownThing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DeletePage(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/unknown/delete?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "unknown")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("", rec.Header().Get("HX-Redirect"))
	is.True(strings.Contains(rec.Body.String(), "thing-v2-delete-dialog"))
}
