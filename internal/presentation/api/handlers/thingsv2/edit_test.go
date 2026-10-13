package thingsv2

import (
	"context"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
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

	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testBinID+"?tenant=t1&mode=edit", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
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
	req := httptest.NewRequest(http.MethodPost, "/things-v2/"+testBinID, strings.NewReader(form.Encode()))
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/"+testBinID+"?tenant=t1", rec.Header().Get("HX-Redirect"))
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
	req := httptest.NewRequest(http.MethodPost, "/things-v2/"+testBinID, strings.NewReader(form.Encode()))
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "name is required"))
}

func TestBuildEditSpecClearsLocationWhenEmpty(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()
	app := &testThingsV2App{svc: svc}

	form := url.Values{
		"tenant":   {"t1"},
		"revision": {"2"},
		"name":     {"Tunna"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things-v2/"+testBinID, strings.NewReader(form.Encode()))
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	spec, _, _, fail := buildEditSpec(req.Context(), app, req, testBinID)
	is.Equal("", fail)
	is.True(spec.Location == nil)
}

func TestComposeEditModelKeepsClearedCoordinates(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()
	app := &testThingsV2App{svc: svc}

	submitted := url.Values{
		"tenant":    {"t1"},
		"latitude":  {""},
		"longitude": {""},
	}
	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testBinID+"?tenant=t1&mode=edit", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	model, err := composeEditModel(req.Context(), req, app, testBinID, submitted, "name is required")
	is.NoErr(err)
	is.Equal("", model.Latitude)
	is.Equal("", model.Longitude)
}

func TestComposeEditModelResolvesPointMode(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()
	app := &testThingsV2App{svc: svc}

	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testBinID+"?tenant=t1&mode=edit", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	model, err := composeEditModel(req.Context(), req, app, testBinID, nil, "")
	is.NoErr(err)
	is.Equal("point", model.GeometryMode)
	is.Equal("", model.GeometryJSON)
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
	req := httptest.NewRequest(http.MethodPost, "/things-v2/"+testBinID, strings.NewReader(form.Encode()))
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Mallen kommer från lagrad config, aldrig från formuläret.
	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/"+testBinID+"?tenant=t1", rec.Header().Get("HX-Redirect"))
}
