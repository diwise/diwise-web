package thingsv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

type testThingsV2App struct {
	svc *appthingsv2.Service
}

func (a *testThingsV2App) ThingsV2() *appthingsv2.Service { return a.svc }

func testLocaleBundle() *ftkmock.LocaleBundleMock {
	return &ftkmock.LocaleBundleMock{
		ForFunc: func(string) frontendtoolkit.Localizer {
			return &ftkmock.LocalizerMock{
				GetFunc:         func(key string) string { return key },
				GetWithDataFunc: func(key string, _ map[string]any) string { return key },
			}
		},
	}
}

func stubThingsV2(t *testing.T) (*appthingsv2.Service, func()) {
	t.Helper()

	mux := http.NewServeMux()
	// Speglar riktiga servern: listan ligger på /things, roten ger 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
		things := []appthingsv2.Thing{
			{
				ThingID: "bin-1", Tenant: "t1", Name: "Tunna",
				Category: "container", TemplateID: "wastebin", TemplateVersion: "v1",
				Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
				Primary:  &appthingsv2.PropertyValue{PropertyID: "fillRate", DisplayName: "Fyllnadsgrad", Value: ptr(42.0), Unit: "%", Quality: "ok"},
			},
			{
				ThingID: "area-1", Tenant: "t1", Name: "Område",
				Category: "area", TemplateID: "area", TemplateVersion: "v1",
				Location: &appthingsv2.Location{Type: "Polygon", Coordinates: json.RawMessage(`[[[17.3,62.39],[17.4,62.39],[17.4,62.4],[17.3,62.39]]]`)},
			},
		}
		if r.URL.Query().Get("name") == "nomatch" {
			things = nil
		}
		if things == nil {
			things = []appthingsv2.Thing{}
		}
		w.Header().Set(appthingsv2.TotalCountHeader, "2")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(things)
	})
	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		specs := []appthingsv2.TemplateSpec{
			{Template: appthingsv2.Template{ID: "wastebin", Version: "v1", Category: "container", DisplayName: "Avfallskärl"}},
			{Template: appthingsv2.Template{ID: "room", Version: "v1", Category: "room", DisplayName: "Rum"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})

	srv := httptest.NewServer(mux)
	return appthingsv2.NewService(&client.Client{}, srv.URL), srv.Close
}

func ptr(v float64) *float64 { return &v }

func TestThingsV2DataListRendersPrimaryAndTenant(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DataList(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?limit=10", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "42%"))
	is.True(strings.Contains(body, `role="progressbar"`))
	is.True(!strings.Contains(body, "Fyllnadsgrad"))
	is.True(strings.Contains(body, "t1"))
	is.True(strings.Contains(body, `id="tableOrMapV2"`))
}

func TestThingsV2DataListMapViewRendersMap(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DataList(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?mapview=true", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), `id="map"`))
}

func TestThingsV2DataListEmptyRendersMissing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DataList(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?name=nomatch", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "missing"))
}

func TestSelectedValuesSplitsCommaSeparatedAndDeduplicates(t *testing.T) {
	is := is.New(t)

	values, err := url.ParseQuery("category=container,room&category=room")
	is.NoErr(err)

	selected := selectedValues(values, "category")

	is.Equal([]string{"container", "room"}, selected)
}

func TestToViewModelKeepsPolygonGeometry(t *testing.T) {
	is := is.New(t)

	viewModel := toViewModel(appthingsv2.Thing{
		ThingID:  "area-1",
		Category: "area",
		Location: &appthingsv2.Location{Type: "Polygon", Coordinates: json.RawMessage(`[[[1,2],[3,4]]]`)},
	})

	is.True(!viewModel.HasLocation)
	is.True(viewModel.HasGeometry)
	is.Equal("Polygon", viewModel.GeometryType)
	is.True(!viewModel.HasPrimaryValue)
}

func TestToViewModelResolvesPointAndPrimary(t *testing.T) {
	is := is.New(t)

	viewModel := toViewModel(appthingsv2.Thing{
		ThingID:  "bin-1",
		Name:     "Tunna",
		Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
		Primary:  &appthingsv2.PropertyValue{PropertyID: "fillRate", Value: ptr(12.5), Unit: "%"},
	})

	is.True(viewModel.HasLocation)
	is.Equal(17.3, viewModel.Longitude)
	is.Equal(62.39, viewModel.Latitude)
	is.True(viewModel.HasPrimaryValue)
	is.Equal("fillRate", viewModel.PrimaryLabel)
	is.Equal(12.5, viewModel.PrimaryValue)
}
