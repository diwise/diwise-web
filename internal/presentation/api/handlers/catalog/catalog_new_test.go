package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func postForm(t *testing.T, handler http.HandlerFunc, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestTemplateNewPagePrefillsCopy(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateNewPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates/new?from=wastebin/v1&tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "wastebin/v1"))
	is.True(strings.Contains(body, "Soptunna"))
	is.True(strings.Contains(body, "fillRate"))
	is.True(strings.Contains(body, `type="submit"`))
	is.True(!strings.Contains(body, `type="button" type="submit"`))
}

func TestTemplateCreatePublishesAndRedirects(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	form := url.Values{
		"tenant": {"t1"}, "id": {"wastebin"}, "version": {"v9"},
		"displayName": {"Soptunna"}, "category": {"waste"},
		"required":      {"fillRate\ntemperature"},
		"geometry":      {"Point", "Polygon"},
		"paramDefaults": {"level=1"},
		"overridable":   {"level"},
	}
	rec := postForm(t, handler, "/catalog/templates/new", form)

	is.Equal(http.StatusFound, rec.Code)
	is.Equal("/catalog/templates/wastebin/v9?tenant=t1", rec.Header().Get("Location"))
}

func TestTemplateCreateConflictShowsFormError(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	// wastebin/v1 finns redan -> 409 -> formulärfel, ingen redirect.
	form := url.Values{"tenant": {"t1"}, "id": {"wastebin"}, "version": {"v1"}}
	rec := postForm(t, handler, "/catalog/templates/new", form)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "versionen finns redan"))
}

func TestTemplateCreateErrorRendersFullPageForBrowserPost(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)
	form := url.Values{"tenant": {"t1"}, "id": {"wastebin"}, "version": {"v1"}}
	req := httptest.NewRequest(http.MethodPost, "/catalog/templates/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "<!doctype html>"))
	is.True(strings.Contains(rec.Body.String(), "diwise.css"))
}

func TestTemplateCreateRejectsBadRef(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	form := url.Values{"tenant": {"t1"}, "id": {"a/b"}, "version": {"v1"}}
	rec := postForm(t, handler, "/catalog/templates/new", form)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "no slashes"))
}

func TestVariantsPageAndDetails(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewVariantsPage(context.Background(), testLocaleBundle(), testAssets(), app)
	req := httptest.NewRequest(http.MethodGet, "/catalog/variants?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "std"))
	is.True(strings.Contains(rec.Body.String(), "wastebin"))

	handler = NewVariantDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)
	req = httptest.NewRequest(http.MethodGet, "/catalog/variants/std/v1?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "std")
	req.SetPathValue("version", "v1")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "level"))
}

func TestVariantCreatePublishesAndRedirects(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewVariantCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	form := url.Values{
		"tenant": {"t1"}, "id": {"std"}, "version": {"v2"},
		"templateRef": {"wastebin/v1"}, "paramValues": {"level=2"},
	}
	rec := postForm(t, handler, "/catalog/variants/new", form)

	is.Equal(http.StatusFound, rec.Code)
	is.Equal("/catalog/variants/std/v2?tenant=t1", rec.Header().Get("Location"))
}
