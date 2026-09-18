package thingsv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingsV2ParentDialogRendersSearch(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentDialog(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parent-dialog?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "thing-v2-parent-dialog"))
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "thing-v2-parent-search"))
	is.True(strings.Contains(body, "revision=2"))
}

func TestThingsV2ParentSearchExcludesSelf(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parents?tenant=t1&query=t&revision=2", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Stubblistan innehåller Tunna (bin-1) + Område (area-1); saken
	// själv ska inte erbjudas som förälder.
	is.True(strings.Contains(body, "Område"))
	is.True(!strings.Contains(body, "Tunna"))
}

func TestThingsV2ParentSearchEmptyQueryRendersNothing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2ParentSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/parents?tenant=t1&revision=2", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "area-1"))
}

func TestThingsV2SetParentRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPut, "/components/things-v2/bin-1/parent?tenant=t1", strings.NewReader("parentId=gh-1&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2SetParentRejectsSelf(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPut, "/components/things-v2/bin-1/parent?tenant=t1", strings.NewReader("parentId=bin-1&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestThingsV2UnlinkParentRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2UnlinkParent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodDelete, "/components/things-v2/bin-1/parent?tenant=t1&revision=2", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}
