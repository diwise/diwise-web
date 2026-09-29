package thingsv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func stubV2(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	// Bas-URL:en innehåller /api/v1-prefixet i drift; stubben svarar på
	// samma relativa paths som klienten anropar ("", "{id}", "catalog/...").
	// Speglar riktiga servern: listan ligger på /things, roten ger 404.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var spec ObjectSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil || spec.ThingID == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(Thing{ThingID: spec.ThingID, Tenant: "t1", Name: spec.Name, Revision: 1})
			return
		}
		tenant := r.URL.Query().Get("tenant")
		if tenant == "nope" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		all := map[string][]Thing{
			"t1": {
				{ThingID: "bin-1", Tenant: "t1", Name: "Tunna", Category: "container",
					TemplateID: "wastebin", TemplateVersion: "v1", Revision: 1,
					Values: map[string]PropertyValue{
						"fillRate": {PropertyID: "fillRate", Value: ptr(42.0), Unit: "%", Quality: "ok"},
					},
					Primary: &PropertyValue{PropertyID: "fillRate", Value: ptr(42.0), Unit: "%", Quality: "ok"}},
			},
			"t2": {
				{ThingID: "room-1", Tenant: "t2", Name: "Rum", Category: "room",
					TemplateID: "room", TemplateVersion: "v1", Revision: 1,
					Location: &Location{Type: "Point", Coordinates: json.RawMessage(`[18.06,59.33]`)},
					Values: map[string]PropertyValue{
						"temperature": {PropertyID: "temperature", Value: ptr(21.5), Unit: "Cel", Quality: "ok"},
					}},
			},
		}
		var things []Thing
		if tenant != "" {
			things = all[tenant]
		} else {
			// Servern fanar ut: slår ihop sorterat över alla tenants.
			for _, ts := range []string{"t1", "t2"} {
				things = append(things, all[ts]...)
			}
		}
		if things == nil {
			things = []Thing{}
		}
		w.Header().Set(TotalCountHeader, "7")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(things)
	})
	mux.HandleFunc("/things/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("If-Match") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var spec ObjectSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1", Name: spec.Name, Revision: 3})
			return
		case http.MethodDelete:
			if r.PathValue("id") == "parent-1" {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1"})
	})
	mux.HandleFunc("/things/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		points := []HistoryPoint{
			{PropertyID: "fillRate", ObservedAt: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC), Value: ptr(42.0), Quality: "ok"},
			{PropertyID: "fillRate", ObservedAt: time.Date(2026, 9, 18, 11, 0, 0, 0, time.UTC), Quality: "uncertain"},
		}
		if r.URL.Query().Get("property") == "missing" {
			points = []HistoryPoint{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(points)
	})
	mux.HandleFunc("/things/{id}/bindings", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPut {
			if r.Header.Get("If-Match") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var body struct {
				Bindings []Binding `json:"bindings"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 3})
			return
		}
		response := map[string][]Binding{"bindings": {
			{DeviceID: "milesight:79", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
		}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})
	mux.HandleFunc("/things/{id}/config", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		config := EffectiveConfig{
			ThingID: "bin-1", Name: "Tunna", TemplateID: "wastebin", TemplateVersion: "v1",
			Bindings:     []Binding{{DeviceID: "milesight:79", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"}},
			ParamValues:  map[string]float64{"sensorToBottom": 0.94},
			ParamSources: map[string]string{"sensorToBottom": "thing"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(config)
	})
	mux.HandleFunc("/things/{id}/overview", func(w http.ResponseWriter, r *http.Request) {
		overview := Overview{Thing: Thing{ThingID: r.PathValue("id"), Tenant: "t1"}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
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
		if body.ParentID == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 3})
	})
	mux.HandleFunc("/things/{id}/parent", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Match") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1", Revision: 4})
	})
	mux.HandleFunc("/catalog/variants", func(w http.ResponseWriter, r *http.Request) {
		specs := []VariantSpec{
			{Variant: Variant{ID: "160L", Version: "v1", TemplateID: "wastebin", TemplateVersion: "v1"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})
	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		specs := []TemplateSpec{
			{Template: Template{ID: "wastebin", Version: "v1", Category: "container", DisplayName: "Soptunna"},
				Recipes: []Recipe{{Name: "fill-rate", Operator: "fillRate", Params: []string{"usableHeight"}}}},
			{Template: Template{ID: "room", Version: "v1", Category: "room", DisplayName: "Rum"}},
		}
		if cat := r.URL.Query().Get("category"); cat != "" {
			filtered := specs[:0]
			for _, s := range specs {
				if s.Template.Category == cat {
					filtered = append(filtered, s)
				}
			}
			specs = filtered
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})
	return httptest.NewServer(mux)
}

func ptr(v float64) *float64 { return &v }

func testService(t *testing.T) (*Service, func()) {
	t.Helper()
	srv := stubV2(t)
	c := &client.Client{}
	svc := NewService(c, srv.URL)
	return svc, srv.Close
}

func TestListThingsParsesArrayAndTotal(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	res, err := svc.ListThings(auth.WithToken(context.Background(), "test-token"), "t1", Filter{Limit: 10})
	is.NoErr(err)
	is.Equal(1, len(res.Things))
	is.Equal(7, res.Total)
	is.Equal("bin-1", res.Things[0].ThingID)
	is.True(res.Things[0].Primary != nil)
	is.Equal(42.0, *res.Things[0].Primary.Value)
}

func TestListThingsAcrossTenantsMergesSorted(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	res, err := svc.ListThingsAcrossTenants(auth.WithToken(context.Background(), "test-token"), Filter{})
	is.NoErr(err)
	is.Equal(2, len(res.Things))
	is.Equal("bin-1", res.Things[0].ThingID)
	is.Equal("room-1", res.Things[1].ThingID)
	is.Equal("t2", res.Things[1].Tenant)
	is.Equal(7, res.Total)
}

func TestListThingsAcrossTenantsFails(t *testing.T) {
	is := is.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := &client.Client{}
	svc := NewService(c, srv.URL)

	_, err := svc.ListThingsAcrossTenants(auth.WithToken(context.Background(), "test-token"), Filter{})
	is.True(err != nil)
}

func TestGetThingNotFound(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	_, err := svc.GetThing(auth.WithToken(context.Background(), "test-token"), "t1", "missing")
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestListTemplatesCategoryFilter(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	specs, err := svc.ListTemplates(auth.WithToken(context.Background(), "test-token"), "t1", "room")
	is.NoErr(err)
	is.Equal(1, len(specs))
	is.Equal("Rum", specs[0].Template.DisplayName)

	specs, err = svc.ListTemplates(auth.WithToken(context.Background(), "test-token"), "t1", "")
	is.NoErr(err)
	is.Equal(2, len(specs))
	is.Equal([]string{"usableHeight"}, specs[0].Recipes[0].Params)
}

func TestLocationPoint(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	res, err := svc.ListThings(auth.WithToken(context.Background(), "test-token"), "t2", Filter{})
	is.NoErr(err)
	lon, lat, ok := res.Things[0].Location.Point()
	is.True(ok)
	is.Equal(18.06, lon)
	is.Equal(59.33, lat)

	var nilLoc *Location
	_, _, ok = nilLoc.Point()
	is.True(!ok)
}

func TestGetHistoryParsesPointsOldestFirst(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	from := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	points, err := svc.GetHistory(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1", "fillRate", from, to, 100)
	is.NoErr(err)
	is.Equal(2, len(points))
	is.Equal("fillRate", points[0].PropertyID)
	is.True(points[0].Value != nil)
	is.Equal(42.0, *points[0].Value)
	is.True(points[1].Value == nil)
	is.True(points[0].ObservedAt.Before(points[1].ObservedAt))
}

func TestGetHistoryEmptyForUnknownProperty(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	points, err := svc.GetHistory(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1", "missing", time.Time{}, time.Time{}, 0)
	is.NoErr(err)
	is.Equal(0, len(points))
}

func TestGetBindingsParsesDeviceSignals(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	bindings, err := svc.GetBindings(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1")
	is.NoErr(err)
	is.Equal(1, len(bindings))
	is.Equal("milesight:79", bindings[0].DeviceID)
	is.Equal("distance", bindings[0].Input)
	is.Equal("urn:oma:lwm2m:ext:3330", bindings[0].Object)
	is.Equal("5700", bindings[0].Resource)
}

func TestGetBindingsNotFound(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	_, err := svc.GetBindings(auth.WithToken(context.Background(), "test-token"), "t1", "missing")
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestGetOverviewParsesThingAndChildren(t *testing.T) {
	is := is.New(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/things/{id}/overview", func(w http.ResponseWriter, r *http.Request) {
		overview := Overview{
			Thing:    Thing{ThingID: "bin-1", Tenant: "t1", Name: "Tunna"},
			Children: []Thing{{ThingID: "room-1", Tenant: "t1", Name: "Rum"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(overview)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)

	overview, err := svc.GetOverview(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1")
	is.NoErr(err)
	is.Equal("Tunna", overview.Thing.Name)
	is.Equal(1, len(overview.Children))
	is.Equal("room-1", overview.Children[0].ThingID)
}

func TestCreateThingPostsSpec(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	thing, err := svc.CreateThing(auth.WithToken(context.Background(), "test-token"), "t1", ObjectSpec{
		ThingID: "bin-9", Name: "Tunna 9",
		Location:        &Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
		TemplateID:      "wastebin",
		TemplateVersion: "v1",
	})
	is.NoErr(err)
	is.Equal("bin-9", thing.ThingID)
	is.Equal("Tunna 9", thing.Name)
	is.Equal(int64(1), thing.Revision)
}

func TestUpdateThingPutsSpecWithRevision(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	thing, err := svc.UpdateThing(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1", ObjectSpec{
		ThingID: "bin-1", Name: "Tunna ny",
		Location:        &Location{Type: "Point", Coordinates: json.RawMessage(`[17.3,62.39]`)},
		TemplateID:      "wastebin",
		TemplateVersion: "v1",
	}, 2)
	is.NoErr(err)
	is.Equal("Tunna ny", thing.Name)
	is.Equal(int64(3), thing.Revision)
}

func TestDeleteThing(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	is.NoErr(svc.DeleteThing(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1"))

	err := svc.DeleteThing(auth.WithToken(context.Background(), "test-token"), "t1", "parent-1")
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrConflict))
}

func TestGetConfigParsesEffectiveConfig(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	config, err := svc.GetConfig(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1")
	is.NoErr(err)
	is.Equal("wastebin", config.TemplateID)
	is.Equal(1, len(config.Bindings))
	is.Equal("milesight:79", config.Bindings[0].DeviceID)
	is.Equal(0.94, config.ParamValues["sensorToBottom"])
	is.Equal("thing", config.ParamSources["sensorToBottom"])

	_, err = svc.GetConfig(auth.WithToken(context.Background(), "test-token"), "t1", "missing")
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestListVariants(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	specs, err := svc.ListVariants(auth.WithToken(context.Background(), "test-token"), "t1")
	is.NoErr(err)
	is.Equal(1, len(specs))
	is.Equal("160L", specs[0].Variant.ID)
	is.Equal("wastebin", specs[0].Variant.TemplateID)
}

func TestMoveParentPutsParentWithRevision(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	thing, err := svc.MoveParent(auth.WithToken(context.Background(), "test-token"), "t1", "tank-1", "gh-1", 2)
	is.NoErr(err)
	is.Equal("tank-1", thing.ThingID)
	is.Equal(int64(3), thing.Revision)
}

func TestMoveParentNotFound(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	_, err := svc.MoveParent(auth.WithToken(context.Background(), "test-token"), "t1", "tank-1", "missing", 2)
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestUnlinkParentDeletesParent(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	thing, err := svc.UnlinkParent(auth.WithToken(context.Background(), "test-token"), "t1", "tank-1", 3)
	is.NoErr(err)
	is.Equal("tank-1", thing.ThingID)
	is.Equal(int64(4), thing.Revision)
}

func TestSwapBindingsPutsBindingsWithRevision(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	thing, err := svc.SwapBindings(auth.WithToken(context.Background(), "test-token"), "t1", "bin-1", []Binding{
		{DeviceID: "milesight:80", Object: "urn:oma:lwm2m:ext:3330", Resource: "5700", Input: "distance"},
	}, 2)
	is.NoErr(err)
	is.Equal("bin-1", thing.ThingID)
	is.Equal(int64(3), thing.Revision)
}

func TestSwapBindingsNotFound(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	_, err := svc.SwapBindings(auth.WithToken(context.Background(), "test-token"), "t1", "missing", nil, 1)
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}
