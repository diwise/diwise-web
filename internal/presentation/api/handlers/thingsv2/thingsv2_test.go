package thingsv2

import (
	"context"
	"encoding/json"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	appthings "github.com/diwise/diwise-web/internal/application/things"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

type testThingsV2App struct {
	svc *appthingsv2.Service
}

const testBinID = "215e3c32-0cfb-4675-b14e-68716480434e"
const testTankID = "05006f98-456a-4c7a-a771-b814b753e4e8"
const testUnknownID = "00000000-0000-4000-8000-000000000001"

func (a *testThingsV2App) ThingsV2() *appthingsv2.Service { return a.svc }

func (a *testThingsV2App) GetValidSensors(_ context.Context, _ []string, _ string) ([]appthings.SensorIdentifier, error) {
	return []appthings.SensorIdentifier{
		{DeviceID: "milesight:80", Name: "Tunnsensor 80"},
		{DeviceID: "milesight:79", Name: "Tunnsensor 79"},
	}, nil
}

func (a *testThingsV2App) GetDevice(_ context.Context, id string) (devices.Device, error) {
	return devices.Device{DeviceID: id, Name: "Namn " + id}, nil
}

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
		if r.Method == http.MethodPost {
			var spec appthingsv2.ObjectSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil || spec.ThingID == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(appthingsv2.Thing{ThingID: spec.ThingID, Tenant: "t1", Name: spec.Name, Revision: 1})
			return
		}
		things := []appthingsv2.Thing{
			{
				ThingID: testBinID, Tenant: "t1", Name: "Tunna",
				Category: "container", TemplateID: "wastebin", TemplateVersion: "v1",
				Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
				Primary:  &appthingsv2.PropertyValue{PropertyID: "fillRate", DisplayName: "Fyllnadsgrad", Value: new(42.0), Unit: "%", Quality: "ok"},
			},
			{
				ThingID: "area-1", Tenant: "t1", Name: "Område",
				Category: "area", TemplateID: "area", TemplateVersion: "v1",
				Location: &appthingsv2.Location{Type: "Polygon", Coordinates: json.RawMessage(`[[[17.3,62.39],[17.4,62.39],[17.4,62.4],[17.3,62.39]]]`)},
			},
			{
				ThingID: "gh-1", Tenant: "t1", Name: "Växthus",
				Category: "greenhouse", TemplateID: "greenhouse", TemplateVersion: "v1",
			},
		}
		if r.URL.Query().Get("name") == "nomatch" {
			things = nil
		}
		if template := r.URL.Query().Get("template"); template != "" {
			filtered := things[:0]
			for _, thing := range things {
				if thing.TemplateID == template {
					filtered = append(filtered, thing)
				}
			}
			things = filtered
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
			{Template: appthingsv2.Template{ID: "wastebin", Version: "v1", Category: "container", DisplayName: "Avfallskärl",
				Required: []string{"distance", "level", "fillRate"},
				PropertyDefs: map[string]appthingsv2.PropertyDef{
					"distance": {DisplayName: "Avstånd", Signals: []appthingsv2.SignalHint{{Object: "urn:oma:lwm2m:ext:3330", Resource: "5700"}}},
					"level":    {DisplayName: "Nivå"},
				}},
				ParamDefaults: map[string]float64{"sensorToBottom": 1.5},
				Overridable:   []string{"sensorToBottom"},
				ParamInfo: map[string]appthingsv2.ParamDef{
					"sensorToBottom": {Unit: "m", Description: "Avstånd sensor till botten", Min: ptr(0), Max: ptr(5)},
				}},
			{Template: appthingsv2.Template{ID: "room", Version: "v1", Category: "room", DisplayName: "Rum"}},
			{Template: appthingsv2.Template{ID: "tank", Version: "v1", Category: "tank", DisplayName: "Tank",
				Relations: []appthingsv2.RelationSpec{{Name: "partOf", AllowedTargets: []string{"greenhouse"}, Max: 1}}}},
			{Template: appthingsv2.Template{ID: "greenhouse", Version: "v1", Category: "greenhouse", DisplayName: "Växthus"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})
	mux.HandleFunc("/catalog/variants", func(w http.ResponseWriter, r *http.Request) {
		specs := []appthingsv2.VariantSpec{
			{Variant: appthingsv2.Variant{ID: "160L", Version: "v1", TemplateID: "wastebin", TemplateVersion: "v1",
				ParamValues: map[string]float64{"usableHeight": 0.79, "sensorToBottom": 0.9}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})
	mux.HandleFunc("/things/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "gh-1" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(appthingsv2.Thing{
				ThingID: "gh-1", Tenant: "t1", Name: "Växthus",
				Category: "greenhouse", TemplateID: "greenhouse", TemplateVersion: "v1", Revision: 1,
			})
			return
		}
		if r.PathValue("id") != testBinID {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		thing := appthingsv2.Thing{
			ThingID: testBinID, Tenant: "t1", Name: "Tunna",
			Category: "container", TemplateID: "wastebin", TemplateVersion: "v1", Revision: 2,
			Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
			Metadata: map[string]string{"subType": "WasteContainer"},
			Values: map[string]appthingsv2.PropertyValue{
				"fillRate": {PropertyID: "fillRate", DisplayName: "Fyllnadsgrad", Value: new(42.0), Unit: "%", Quality: "ok"},
				"level":    {PropertyID: "level", DisplayName: "Nivå", Value: new(0.5), Unit: "m", Quality: "ok"},
			},
			Primary: &appthingsv2.PropertyValue{PropertyID: "fillRate", DisplayName: "Fyllnadsgrad", Value: new(42.0), Unit: "%", Quality: "ok"},
		}
		if r.Method == http.MethodPut {
			if r.Header.Get("If-Match") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var spec appthingsv2.ObjectSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if spec.TemplateID != "wastebin" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			thing.Name = spec.Name
			thing.Revision = 3
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(thing)
	})
	mux.HandleFunc("/things/{id}/config", func(w http.ResponseWriter, r *http.Request) {
		var config appthingsv2.EffectiveConfig
		switch r.PathValue("id") {
		case testTankID:
			config = appthingsv2.EffectiveConfig{
				ThingID: testTankID, TemplateID: "tank", TemplateVersion: "v1",
			}
		case testBinID:
			config = appthingsv2.EffectiveConfig{
				ThingID: testBinID, Name: "Tunna", Category: "container",
				TemplateID: "wastebin", TemplateVersion: "v1",
				Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
				Metadata: map[string]string{"subType": "WasteContainer"},
				Bindings: []appthingsv2.Binding{
					{DeviceID: "milesight:79", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
				},
				ParamValues:  map[string]float64{"sensorToBottom": 0.94},
				ParamSources: map[string]string{"sensorToBottom": "thing"},
				Relations: []appthingsv2.RelationRef{
					{Name: "partOf", TargetThingID: "gh-1", TargetTemplateID: "greenhouse"},
				},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config)
	})
	mux.HandleFunc("/things/{id}/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Match") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct {
			ParentID string `json:"parentId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ParentID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(appthingsv2.Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 3})
	})
	mux.HandleFunc("/things/{id}/parent", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Match") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(appthingsv2.Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 4})
	})
	mux.HandleFunc("/things/{id}/bindings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			if r.Header.Get("If-Match") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var body struct {
				Bindings []appthingsv2.Binding `json:"bindings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(appthingsv2.Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 3})
			return
		}
		response := map[string][]appthingsv2.Binding{"bindings": {
			{DeviceID: "milesight:79", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/things/{id}/overview", func(w http.ResponseWriter, r *http.Request) {
		overview := appthingsv2.Overview{
			Thing: appthingsv2.Thing{ThingID: testBinID, Tenant: "t1", Name: "Tunna"},
			Children: []appthingsv2.Thing{
				{ThingID: "room-1", Tenant: "t1", Name: "Rum", Category: "room"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
	})
	mux.HandleFunc("/things/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]string{"centrum", "park"})
	})
	mux.HandleFunc("/things/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		points := []appthingsv2.HistoryPoint{
			{PropertyID: "fillRate", ObservedAt: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC), Value: new(42.0), Quality: "ok"},
			{PropertyID: "fillRate", ObservedAt: time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC), Quality: "uncertain"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(points)
	})

	srv := httptest.NewServer(mux)
	return appthingsv2.NewService(&client.Client{}, srv.URL), srv.Close
}

//go:fix inline
func ptr(v float64) *float64 { return new(v) }

type stubAsset struct{ path string }

func (a stubAsset) Body() []byte        { return nil }
func (a stubAsset) ContentLength() int  { return 0 }
func (a stubAsset) ContentType() string { return "text/plain" }
func (a stubAsset) Path() string        { return a.path }
func (a stubAsset) SHA256() string      { return "" }

func testAssets() frontendtoolkit.AssetLoaderFunc {
	return func(name string) frontendtoolkit.Asset { return stubAsset{path: name} }
}

func TestThingsV2DataListRendersPrimaryAndTenant(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DataList(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?limit=10", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
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

func TestThingsV2DataListRendersTagFilter(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()
	app := &testThingsV2App{svc: svc}

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?limit=10&tag=park", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))

	model, err := composeListModel(req.Context(), req, testLocaleBundle().For(""), app)
	is.NoErr(err)
	is.Equal([]string{"park"}, model.Filters.SelectedTags)
	is.Equal(2, len(model.TagOptions))
	is.Equal("centrum", model.TagOptions[0].Value)
	is.Equal("park", model.TagOptions[1].Value)
}

func TestThingsV2DataListMapViewRendersMap(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DataList(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/list?mapview=true", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
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
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
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
		ThingID:  testBinID,
		Name:     "Tunna",
		Location: &appthingsv2.Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
		Primary:  &appthingsv2.PropertyValue{PropertyID: "fillRate", Value: new(12.5), Unit: "%"},
	})

	is.True(viewModel.HasLocation)
	is.Equal(17.3, viewModel.Longitude)
	is.Equal(62.39, viewModel.Latitude)
	is.True(viewModel.HasPrimaryValue)
	is.Equal("fillRate", viewModel.PrimaryLabel)
	is.Equal(12.5, viewModel.PrimaryValue)
}

func TestThingsV2DetailsPageRendersValuesAndMetadata(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DetailsPage(context.Background(), testLocaleBundle(), testAssets(), &testThingsV2App{svc: svc})

	// HX-vägen renderar AppShell direkt; full sida kräver inloggad ctx.
	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testBinID+"?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Tunna"))
	is.True(strings.Contains(body, "Fyllnadsgrad"))
	is.True(strings.Contains(body, "42 %"))
	is.True(strings.Contains(body, "WasteContainer"))
	is.True(strings.Contains(body, `role="progressbar"`))
	is.True(strings.Contains(body, "milesight:79"))
	is.True(strings.Contains(body, "/sensors/milesight:79"))
	is.True(strings.Contains(body, "/things-v2/room-1"))
	is.True(strings.Contains(body, "/things-v2/gh-1"))
}

func TestThingsV2DetailsPageReturns404ForUnknownThing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DetailsPage(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testUnknownID+"?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.SetPathValue("id", testUnknownID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNotFound, rec.Code)
}

func TestThingsV2HandlersRejectInvalidIDWithoutUpstreamRequests(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	}))
	defer upstream.Close()
	app := &testThingsV2App{svc: appthingsv2.NewService(&client.Client{}, upstream.URL)}
	ctx := context.Background()
	l10n := testLocaleBundle()
	assets := testAssets()
	for _, tc := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		{"details", "GET", "/things-v2/{id}", NewThingsV2DetailsPage(ctx, l10n, assets, app)},
		{"history", "GET", "/components/things-v2/{id}/history", NewThingsV2HistoryComponent(ctx, l10n, assets, app)},
		{"save", "POST", "/things-v2/{id}", NewThingsV2SavePage(ctx, l10n, assets, app)},
		{"delete-dialog", "GET", "/components/things-v2/{id}/delete", NewThingsV2DeleteDialog(ctx, l10n, assets, app)},
		{"delete", "POST", "/components/things-v2/{id}/delete", NewThingsV2DeletePage(ctx, l10n, assets, app)},
		{"parent-dialog", "GET", "/components/things-v2/{id}/parent-dialog", NewThingsV2ParentDialog(ctx, l10n, assets, app)},
		{"parent-search", "GET", "/components/things-v2/{id}/parents", NewThingsV2ParentSearch(ctx, l10n, assets, app)},
		{"set-parent", "POST", "/components/things-v2/{id}/parent", NewThingsV2SetParent(ctx, l10n, assets, app)},
		{"unlink-parent", "POST", "/components/things-v2/{id}/parent/remove", NewThingsV2UnlinkParent(ctx, l10n, assets, app)},
		{"sensors-dialog", "GET", "/components/things-v2/{id}/sensors", NewThingsV2SensorsDialog(ctx, l10n, assets, app)},
		{"sensor-search", "GET", "/components/things-v2/{id}/sensors/search", NewThingsV2SensorSearch(ctx, l10n, assets, app)},
		{"set-sensor", "POST", "/components/things-v2/{id}/sensors", NewThingsV2SetSensor(ctx, l10n, assets, app)},
		{"unbind-sensor", "POST", "/components/things-v2/{id}/sensors/unbind", NewThingsV2UnbindSensor(ctx, l10n, assets, app)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc(tc.method+" "+tc.path, tc.handler)
			for _, id := range []string{"undefined", "null", "not-a-uuid", "215e3c32-0cfb-4675-b14e-68716480434z"} {
				t.Run(id, func(t *testing.T) {
					path := strings.ReplaceAll(tc.path, "{id}", id)
					req := httptest.NewRequest(tc.method, path+"?tenant=t1&property=fillRate&input=distance&query=test", strings.NewReader("revision=2&parentId="+testTankID+"&input=distance&deviceId=test"))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, req)
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("status = %d, want 400", rec.Code)
					}
					if calls.Load() != 0 {
						t.Fatalf("upstream requests = %d, want 0", calls.Load())
					}
				})
			}
		})
	}
}

func TestThingsV2DetailsPageRequiresTenantWhenAmbiguous(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2DetailsPage(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/things-v2/"+testBinID, nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestToDetailsViewModelSortsValuesAndMetadata(t *testing.T) {
	is := is.New(t)

	model := toDetailsViewModel(appthingsv2.Thing{
		ThingID:         testBinID,
		TemplateID:      "wastebin",
		TemplateVersion: "v1",
		Revision:        2,
		Values: map[string]appthingsv2.PropertyValue{
			"level":    {PropertyID: "level", Value: new(0.5)},
			"distance": {PropertyID: "distance"},
		},
		Metadata: map[string]string{"b": "2", "a": "1"},
	}, []appthingsv2.Binding{
		{DeviceID: "milesight:79", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
		{DeviceID: "milesight:12", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
	}, []appthingsv2.Thing{
		{ThingID: "room-1", Tenant: "t1", Name: "Rum", Category: "room"},
		{ThingID: testBinID},
	}, &appthingsv2.Thing{ThingID: "gh-1", Tenant: "t1", Name: "Växthus"})

	is.Equal(2, len(model.Values))
	is.Equal("distance", model.Values[0].PropertyID)
	is.True(!model.Values[0].HasValue)
	is.Equal("level", model.Values[1].PropertyID)
	is.True(model.Values[1].HasValue)
	is.Equal(2, len(model.Metadata))
	is.Equal("a", model.Metadata[0].Key)
	is.Equal(int64(2), model.Revision)
	is.Equal(2, len(model.ConnectedSensors))
	is.Equal("milesight:12", model.ConnectedSensors[0].DeviceID)
	is.Equal("distance", model.ConnectedSensors[0].Input)
	is.Equal(1, len(model.RelatedThings))
	is.Equal("room-1", model.RelatedThings[0].ID)
	is.True(model.Parent != nil)
	is.Equal("gh-1", model.Parent.ID)
	is.Equal("Växthus", model.Parent.Name)
}

func TestThingsV2HistoryComponentRendersChart(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2HistoryComponent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/"+testBinID+"/history?tenant=t1&property=fillRate&span=today", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "Fyllnadsgrad"))
	is.True(strings.Contains(body, "2026-09-18 10:00"))
	is.True(strings.Contains(body, "<canvas"))
}

func TestThingsV2HistoryComponentRequiresProperty(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2HistoryComponent(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/"+testBinID+"/history?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", testBinID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestHistorySpanRangeDefaultsToToday(t *testing.T) {
	is := is.New(t)

	now := time.Date(2026, 9, 18, 14, 30, 0, 0, time.UTC)

	from, to := historySpanRange("today", now)
	is.Equal(time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), from)
	is.Equal(now, to)

	from, to = historySpanRange("bogus", now)
	is.Equal(time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), from)
	is.Equal(now, to)

	from, to = historySpanRange("24h", now)
	is.Equal(now.Add(-24*time.Hour), from)
	is.Equal(now, to)

	from, _ = historySpanRange("7d", now)
	is.Equal(now.Add(-7*24*time.Hour), from)

	from, _ = historySpanRange("30d", now)
	is.Equal(now.Add(-30*24*time.Hour), from)
}
