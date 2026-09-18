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

func TestThingsV2EditModeRendersLockedTemplate(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DetailsPage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/bin-1?tenant=t1&mode=edit", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "Avfallskärl v1"))
	is.True(strings.Contains(body, "0.94"))
	is.True(strings.Contains(body, `name="revision" value="2"`))
	// Befintlig plats ska synas som flyttbar markör i kartan.
	is.True(strings.Contains(body, "[17.3,62.39]"))
}

func TestThingsV2SavePageUpdatesAndRedirects(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SavePage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	form := url.Values{
		"tenant":               {"t1"},
		"revision":             {"2"},
		"name":                 {"Tunna ny"},
		"latitude":             {"62.39"},
		"longitude":            {"17.3"},
		"param.sensorToBottom": {"1.0"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/bin-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2SavePageRejectsMissingName(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SavePage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	form := url.Values{
		"tenant":    {"t1"},
		"revision":  {"2"},
		"latitude":  {"62.39"},
		"longitude": {"17.3"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/bin-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "name is required"))
}

func TestThingsV2SavePageIgnoresSmuggledTemplate(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SavePage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	form := url.Values{
		"tenant":    {"t1"},
		"revision":  {"2"},
		"template":  {"room/v1"},
		"name":      {"Tunna"},
		"latitude":  {"62.39"},
		"longitude": {"17.3"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/bin-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Mallen kommer från lagrad config, aldrig från formuläret.
	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}
