package thingsv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingsV2NewPageShowsTemplatePicker(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2NewPage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/new", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "wastebin/v1"))
	is.True(!strings.Contains(body, `name="thingId"`))
}

func TestThingsV2NewPageShowsFormForTemplate(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2NewPage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/new?template=wastebin/v1", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, `name="thingId"`))
	is.True(strings.Contains(body, `name="param.sensorToBottom"`))
	is.True(strings.Contains(body, "160L"))
}

func TestThingsV2CreatePageCreatesAndRedirects(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2CreatePage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	form := url.Values{
		"tenant":               {"t1"},
		"template":             {"wastebin/v1"},
		"thingId":              {"bin-9"},
		"name":                 {"Tunna 9"},
		"latitude":             {"62.39"},
		"longitude":            {"17.3"},
		"param.sensorToBottom": {"0.94"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-9?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2CreatePageRejectsMissingName(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2CreatePage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	form := url.Values{
		"tenant":    {"t1"},
		"template":  {"wastebin/v1"},
		"thingId":   {"bin-9"},
		"latitude":  {"62.39"},
		"longitude": {"17.3"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "name is required"))
	is.True(strings.Contains(body, "bin-9"))
}

func TestSplitTemplateRef(t *testing.T) {
	is := is.New(t)

	id, version, ok := splitTemplateRef("wastebin/v1")
	is.True(ok)
	is.Equal("wastebin", id)
	is.Equal("v1", version)

	for _, raw := range []string{"", "wastebin", "/v1", "wastebin/", "a/b/c"} {
		_, _, ok = splitTemplateRef(raw)
		is.True(!ok)
	}
}
