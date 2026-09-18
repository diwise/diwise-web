package thingsv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
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
	mux.HandleFunc("/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Thing{ThingID: r.PathValue("id"), Tenant: "t1"})
	})
	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		specs := []TemplateSpec{
			{Template: Template{ID: "wastebin", Version: "v1", Category: "container", DisplayName: "Soptunna"}},
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

	res, err := svc.ListThings(context.Background(), "t1", Filter{Limit: 10})
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

	res, err := svc.ListThingsAcrossTenants(context.Background(), Filter{})
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

	_, err := svc.ListThingsAcrossTenants(context.Background(), Filter{})
	is.True(err != nil)
}

func TestGetThingNotFound(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	_, err := svc.GetThing(context.Background(), "t1", "missing")
	is.True(err != nil)
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestListTemplatesCategoryFilter(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	specs, err := svc.ListTemplates(context.Background(), "t1", "room")
	is.NoErr(err)
	is.Equal(1, len(specs))
	is.Equal("Rum", specs[0].Template.DisplayName)

	specs, err = svc.ListTemplates(context.Background(), "t1", "")
	is.NoErr(err)
	is.Equal(2, len(specs))
}

func TestLocationPoint(t *testing.T) {
	is := is.New(t)
	svc, done := testService(t)
	defer done()

	res, err := svc.ListThings(context.Background(), "t2", Filter{})
	is.NoErr(err)
	lon, lat, ok := res.Things[0].Location.Point()
	is.True(ok)
	is.Equal(18.06, lon)
	is.Equal(59.33, lat)

	var nilLoc *Location
	_, _, ok = nilLoc.Point()
	is.True(!ok)
}
